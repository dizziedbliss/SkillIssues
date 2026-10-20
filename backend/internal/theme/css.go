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
//
// @property may register only --theme-* properties: a theme can animate its
// own custom properties, never change how the app's tokens (--colors-…)
// inherit. packages/theme lifts these rules out of the app's @scope.

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
		cubic-bezier steps linear scroll view
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
	line := 1 // of the token being read
	add := func(format string, args ...any) {
		if len(problems) < 20 {
			problems = append(problems, fmt.Sprintf("stylesheet line %d: ", line)+fmt.Sprintf(format, args...))
		}
	}
	if i := strings.IndexRune(src, '\\'); i >= 0 {
		line = 1 + strings.Count(src[:i], "\n")
		add("escapes (\\) are not allowed")
	}
	if i := strings.IndexRune(src, 0); i >= 0 {
		line = 1 + strings.Count(src[:i], "\n")
		add("NUL bytes are not allowed")
	}
	line = 1

	l := css.NewLexer(parse.NewInputString(src))
	afterURLFunction := false
	afterProperty := false
	next := 1 // the line the next token starts on
	for {
		tt, data := l.Next()
		line = next
		next += strings.Count(string(data), "\n")
		if tt == css.ErrorToken {
			if l.Err() != io.EOF {
				add("unreadable CSS: %v", l.Err())
			}
			break
		}
		text := strings.ToLower(string(data))
		if afterProperty && tt != css.WhitespaceToken {
			afterProperty = false
			if !strings.HasPrefix(text, "--theme-") {
				add("@property may only register a --theme-* property")
			}
			continue
		}
		if afterURLFunction && tt != css.WhitespaceToken {
			afterURLFunction = false
			if tt != css.StringToken || !dataImage.MatchString(strings.Trim(string(data), `"'`)) {
				add("url() may only hold a data:image (png, jpeg, gif or webp) in base64")
			}
			continue
		}
		switch tt {
		case css.AtKeywordToken:
			if text == "@property" {
				afterProperty = true
			} else if !allowedAtRules[text] {
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
