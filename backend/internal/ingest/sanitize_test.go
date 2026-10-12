package ingest

import "testing"

func TestSanitizeText(t *testing.T) {
	cases := map[string]string{
		"clean title":     "clean title",
		"has\x00nul":      "hasnul",
		"\x00\x00leading": "leading",
		"trailing\x00":    "trailing",
		"":                "",
	}
	for in, want := range cases {
		if got := sanitizeText(in); got != want {
			t.Errorf("sanitizeText(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSanitizeSlice(t *testing.T) {
	got := sanitizeSlice([]string{"a\x00b", "clean", ""})
	want := []string{"ab", "clean", ""}
	if len(got) != len(want) {
		t.Fatalf("length mismatch: got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("index %d: got %q want %q", i, got[i], want[i])
		}
	}
}
