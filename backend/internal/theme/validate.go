package theme

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

// ErrInvalid is wrapped by every reason a theme is refused.
var ErrInvalid = errors.New("theme: invalid")

// Problems lists everything wrong with a theme, so an author fixes it in one
// pass rather than one error per save.
type Problems []string

func (p Problems) Error() string {
	return fmt.Sprintf("%v: %s", ErrInvalid, strings.Join(p, "; "))
}

func (p Problems) Unwrap() error { return ErrInvalid }

// Tokens is a validated theme file: table → key → value.
type Tokens map[string]map[string]any

// ParseTOML parses and validates a theme file against Schema.
func ParseTOML(src string) (Tokens, error) {
	var raw map[string]any
	if _, err := toml.Decode(src, &raw); err != nil {
		return nil, Problems{"not valid TOML: " + err.Error()}
	}
	var problems Problems
	out := Tokens{}
	tables := make([]string, 0, len(raw))
	for t := range raw {
		tables = append(tables, t)
	}
	sort.Strings(tables)
	for _, t := range tables {
		spec, ok := Schema[t]
		if !ok {
			problems = append(problems, fmt.Sprintf("[%s] is not a theme table", t))
			continue
		}
		values, ok := raw[t].(map[string]any)
		if !ok {
			problems = append(problems, fmt.Sprintf("%s must be a table ([%s])", t, t))
			continue
		}
		keys := make([]string, 0, len(values))
		for k := range values {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		out[t] = map[string]any{}
		for _, k := range keys {
			kd, ok := spec[k]
			if !ok {
				problems = append(problems, fmt.Sprintf("%s.%s is not a theme key", t, k))
				continue
			}
			if !kd.check(values[k]) {
				problems = append(problems, fmt.Sprintf("%s.%s is not a valid %s", t, k, kd.name))
				continue
			}
			out[t][k] = values[k]
		}
	}
	if len(problems) > 0 {
		return nil, problems
	}
	return out, nil
}

// Name is the theme's meta.name, or "" when it has none.
func (t Tokens) Name() string {
	if s, ok := t["meta"]["name"].(string); ok {
		return strings.TrimSpace(s)
	}
	return ""
}
