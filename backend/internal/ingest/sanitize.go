package ingest

import "strings"

// SanitizeText strips NUL bytes, which Postgres's text type rejects
// outright — a single NUL anywhere in a batch would otherwise fail the
// whole page's transaction (plan §7, §4 "sanitize nested strings, including
// NUL bytes, immediately before database persistence").
//
// Exported because the same rule applies to every writer of GitHub-sourced
// text, not just the issue path: cmd/repoimport hits real NULs in
// repository descriptions and topic names.
func SanitizeText(s string) string {
	if !strings.ContainsRune(s, 0) {
		return s
	}
	return strings.ReplaceAll(s, "\x00", "")
}

// SanitizeSlice applies SanitizeText to every element.
func SanitizeSlice(ss []string) []string {
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = SanitizeText(s)
	}
	return out
}

// SanitizeOptional applies SanitizeText through a pointer, preserving the
// nil/NULL distinction — an absent description must stay absent rather than
// becoming an empty string.
func SanitizeOptional(s *string) *string {
	if s == nil {
		return nil
	}
	cleaned := SanitizeText(*s)
	return &cleaned
}
