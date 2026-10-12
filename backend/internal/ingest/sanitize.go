package ingest

import "strings"

// sanitizeText strips NUL bytes, which Postgres's text type rejects
// outright — a single NUL anywhere in a batch would otherwise fail the
// whole page's transaction (plan §7, §4 "sanitize nested strings, including
// NUL bytes, immediately before database persistence").
func sanitizeText(s string) string {
	if !strings.ContainsRune(s, 0) {
		return s
	}
	return strings.ReplaceAll(s, "\x00", "")
}

func sanitizeSlice(ss []string) []string {
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = sanitizeText(s)
	}
	return out
}
