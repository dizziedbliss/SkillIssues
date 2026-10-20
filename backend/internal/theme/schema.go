// Package theme validates and stores user themes: a TOML file of design
// tokens and an optional custom stylesheet, BetterDiscord-style. A theme
// restyles the signed-in app only; the landing page is never themed.
//
// Themes can be shared by link, so a theme is someone else's code running
// on the viewer's page. Every value is checked against a fixed schema
// (schema.go) and the stylesheet against an allowlist (css.go); the web
// app's Content-Security-Policy is the second line. packages/theme mirrors
// both in TypeScript and runs the same fixtures (packages/theme/fixtures).
package theme

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// SchemaVersion is the theme file format this server accepts.
const SchemaVersion = 1

// kind is how a token's value is checked.
type kind struct {
	name  string
	check func(any) bool
}

func str(pred func(string) bool) func(any) bool {
	return func(v any) bool {
		s, ok := v.(string)
		return ok && pred(strings.TrimSpace(s))
	}
}

var (
	hexColor  = regexp.MustCompile(`^#(?:[0-9a-fA-F]{3,4}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})$`)
	funcColor = regexp.MustCompile(`^(?:rgb|rgba|hsl|hsla|oklch|oklab)\(\s*[0-9.%deg\s,/+-]+\)$`)
	namedClr  = map[string]bool{"transparent": true, "currentcolor": true, "black": true, "white": true}
	lengthRe  = regexp.MustCompile(`^(-?\d+(?:\.\d+)?)(px|rem|em)$`)
	durRe     = regexp.MustCompile(`^(\d+)ms$`)
	shadowRe  = regexp.MustCompile(`^(?:none|(?:inset\s+)?(?:-?\d+(?:\.\d+)?px\s+){2,4}(?:#[0-9a-fA-F]{3,8}|(?:rgb|rgba|hsl|hsla|oklch)\([0-9.%\s,/+-]+\)))$`)
	dataImage = regexp.MustCompile(`^data:image/(?:png|jpeg|gif|webp);base64,[A-Za-z0-9+/]+={0,2}$`)
)

func isColor(s string) bool {
	return hexColor.MatchString(s) || funcColor.MatchString(s) || namedClr[strings.ToLower(s)]
}

// lengthIn accepts px between lo and hi, or rem/em between lo/16 and hi/16.
func lengthIn(lo, hi float64) func(any) bool {
	return str(func(s string) bool {
		m := lengthRe.FindStringSubmatch(s)
		if m == nil {
			return false
		}
		n, err := strconv.ParseFloat(m[1], 64)
		if err != nil {
			return false
		}
		if m[2] != "px" {
			n *= 16
		}
		return n >= lo && n <= hi
	})
}

func numberIn(lo, hi float64) func(any) bool {
	return func(v any) bool {
		var n float64
		switch x := v.(type) {
		case int64:
			n = float64(x)
		case float64:
			n = x
		default:
			return false
		}
		return n >= lo && n <= hi
	}
}

func oneOf(vals ...string) func(any) bool {
	return str(func(s string) bool { return slices.Contains(vals, s) })
}

func textUpTo(n int) func(any) bool {
	return str(func(s string) bool { return len(s) <= n && !strings.ContainsAny(s, "<>\x00") })
}

// Fonts a theme may name. They are self-hosted by the web app, so a theme
// never makes the page fetch a font from elsewhere.
var Fonts = []string{"Instrument Serif", "JetBrains Mono", "Krona One", "system-ui", "serif", "sans-serif", "monospace"}

// Presets a theme may start from (meta.base).
var Presets = []string{"default", "midnight", "terminal", "contrast"}

var (
	color    = kind{"color", str(isColor)}
	font     = kind{"font", oneOf(Fonts...)}
	image    = kind{"image", str(func(s string) bool { return s == "none" || dataImage.MatchString(s) })}
	shadow   = kind{"shadow", str(shadowRe.MatchString)}
	duration = kind{"duration", str(func(s string) bool {
		m := durRe.FindStringSubmatch(s)
		if m == nil {
			return false
		}
		n, _ := strconv.Atoi(m[1])
		return n <= 2000
	})}
	boolean = kind{"boolean", func(v any) bool { _, ok := v.(bool); return ok }}
)

func length(lo, hi float64) kind { return kind{"length", lengthIn(lo, hi)} }
func number(lo, hi float64) kind { return kind{"number", numberIn(lo, hi)} }
func enum(vals ...string) kind   { return kind{"enum", oneOf(vals...)} }
func text(n int) kind            { return kind{"text", textUpTo(n)} }

// Schema is every table and key a theme file may hold. A key not listed is
// an error, so a typo is reported instead of silently ignored. Each becomes
// the CSS custom property --<table>-<key> (tokens.go in packages/theme).
var Schema = map[string]map[string]kind{
	"meta": {
		"name": text(60), "author": text(60), "version": text(20), "description": text(280),
		"base":   enum(Presets...),
		"schema": number(SchemaVersion, SchemaVersion),
	},
	"colors": {
		"background": color, "surface": color, "surface-raised": color, "text": color, "text-muted": color,
		"accent": color, "accent-text": color, "border": color, "focus": color,
		"success": color, "warning": color, "danger": color, "selection": color,
	},
	"background": {
		"image": image, "pattern": enum("none", "grid", "dots", "lines"), "pattern-color": color,
		"pattern-size": length(4, 96),
	},
	"typography": {
		"heading-font": font, "body-font": font, "mono-font": font,
		"base-size": length(12, 22), "scale": number(1, 1.6), "line-height": number(1, 2),
		"heading-weight": number(100, 900), "body-weight": number(100, 900),
	},
	"spacing": {
		"unit": length(2, 8), "density": enum("compact", "comfortable", "spacious"),
		"page-gutter": length(8, 96), "max-width": length(640, 2400),
	},
	"radius": {"small": length(0, 48), "medium": length(0, 48), "large": length(0, 48), "pill": length(0, 999)},
	"border": {"width": length(0, 4), "style": enum("solid", "dashed", "dotted", "none")},
	"shadow": {"small": shadow, "medium": shadow, "large": shadow},
	"motion": {"duration": duration, "easing": enum("linear", "ease", "ease-in", "ease-out", "ease-in-out"), "reduce": boolean},
}
