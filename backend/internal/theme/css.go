package theme

import (
	"fmt"
	"io"
	"strings"

	"github.com/tdewolff/parse/v2"
	"github.com/tdewolff/parse/v2/css"
)

// A theme's stylesheet runs on the page of whoever applies the theme, so it
// is checked token by token against an allowlist rather than a list of known
// tricks. What it must not do is make the browser fetch anything (a request
// carrying page state out: url(), @import, @font-face, image-set() with a
// string) or run anything (expression(), behavior, -moz-binding). Escapes
// are refused outright, since u\72l( spells url( to the browser and not to a
// filter. The web app's CSP blocks the same fetches a second time.

var allowedAtRules = map[string]bool{
	"@media": true, "@supports": true, "@keyframes": true, "@container": true, "@layer": true,
}

var allowedFunctions = func() map[string]bool {
	m := map[string]bool{}
	for f := range strings.FieldsSeq(`
		calc var min max clamp round mod rem abs sign sin cos tan
		rgb rgba hsl hsla hwb lab lch oklab oklch color color-mix light-dark
		linear-gradient radial-gradient conic-gradient
		repeating-linear-gradient repeating-radial-gradient repeating-conic-gradient
		translate translatex translatey translatez translate3d rotate rotatex rotatey rotatez rotate3d
		scale scalex scaley scalez scale3d skew skewx skewy matrix matrix3d perspective
		cubic-bezier steps linear
		blur brightness contrast drop-shadow grayscale hue-rotate invert opacity saturate sepia
		repeat minmax fit-content counter counters
		inset circle ellipse polygon
		not is where has nth-child nth-last-child nth-of-type nth-last-of-type lang dir`) {
		m[f] = true
	}
	return m
}()

var deniedIdents = map[string]bool{"behavior": true, "-ms-behavior": true, "-moz-binding": true}

// CheckCSS reports every reason a theme stylesheet is refused; nil when it
// is acceptable.
func CheckCSS(src string) error {
	var problems Problems
	add := func(format string, args ...any) {
		if len(problems) < 20 {
			problems = append(problems, fmt.Sprintf(format, args...))
		}
	}
	if strings.ContainsRune(src, '\\') {
		add("escapes (\\) are not allowed")
	}
	if strings.ContainsRune(src, 0) {
		add("NUL bytes are not allowed")
	}

	l := css.NewLexer(parse.NewInputString(src))
	afterURLFunction := false
	for {
		tt, data := l.Next()
		if tt == css.ErrorToken {
			if l.Err() != io.EOF {
				add("unreadable CSS: %v", l.Err())
			}
			break
		}
		text := strings.ToLower(string(data))
		if afterURLFunction && tt != css.WhitespaceToken {
			afterURLFunction = false
			if tt != css.StringToken || !dataImage.MatchString(strings.Trim(string(data), `"'`)) {
				add("url() may only hold a data:image (png, jpeg, gif or webp) in base64")
			}
			continue
		}
		switch tt {
		case css.AtKeywordToken:
			if !allowedAtRules[text] {
				add("%s is not allowed", text)
			}
		case css.URLToken:
			inner := strings.TrimSpace(string(data[4 : len(data)-1]))
			if !dataImage.MatchString(strings.Trim(inner, `"'`)) {
				add("url() may only hold a data:image (png, jpeg, gif or webp) in base64")
			}
		case css.FunctionToken:
			name := strings.TrimSuffix(text, "(")
			if name == "url" {
				afterURLFunction = true
			} else if !allowedFunctions[name] {
				add("%s() is not allowed", name)
			}
		case css.IdentToken:
			if deniedIdents[text] {
				add("%s is not allowed", text)
			}
		case css.StringToken:
			if strings.Contains(text, "javascript:") {
				add("javascript: is not allowed")
			}
		case css.BadURLToken, css.BadStringToken:
			add("malformed %s", tt)
		case css.CDOToken, css.CDCToken:
			add("HTML comment markers are not allowed")
		case css.DelimToken:
			if text == "<" {
				add("< is not allowed")
			}
		}
	}
	if len(problems) > 0 {
		return problems
	}
	return nil
}
