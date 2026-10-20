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

// lines maps "[table]" and "table.key" to the line (from 1) where a theme
// file writes them, so a problem can say where to look. It reads the file's
// own layout (a [table] header, then key = value lines), which is all a
// theme file holds.
func lines(src string) map[string]int {
	out := map[string]int{}
	table := ""
	for i, line := range strings.Split(src, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]"):
			table = strings.TrimSpace(strings.Trim(line, "[]"))
			if _, ok := out["["+table+"]"]; !ok {
				out["["+table+"]"] = i + 1
			}
		case strings.Contains(line, "=") && !strings.HasPrefix(line, "#"):
			key := strings.Trim(strings.TrimSpace(line[:strings.Index(line, "=")]), `"'`)
			if _, ok := out[table+"."+key]; !ok {
				out[table+"."+key] = i + 1
			}
		}
	}
	return out
}

// at prefixes a problem with its line, when the file says where.
func at(where map[string]int, ref, problem string) string {
	if n, ok := where[ref]; ok {
		return fmt.Sprintf("line %d: %s", n, problem)
	}
	return problem
}

// ParseTOML parses and validates a theme file against Schema.
func ParseTOML(src string) (Tokens, error) {
	var raw map[string]any
	if _, err := toml.Decode(src, &raw); err != nil {
		return nil, Problems{"not valid TOML: " + err.Error()}
	}
	var problems Problems
	where := lines(src)
	out := Tokens{}
	tables := make([]string, 0, len(raw))
	for t := range raw {
		tables = append(tables, t)
	}
	sort.Strings(tables)
	for _, t := range tables {
		spec, ok := Schema[t]
		if !ok {
			problems = append(problems, at(where, "["+t+"]", fmt.Sprintf("[%s] is not a theme table", t)))
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
				problems = append(problems, at(where, t+"."+k, fmt.Sprintf("%s.%s is not a theme key", t, k)))
				continue
			}
			if !kd.check(values[k]) {
				problems = append(problems, at(where, t+"."+k, fmt.Sprintf("%s.%s is not a valid %s", t, k, kd.name)))
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
