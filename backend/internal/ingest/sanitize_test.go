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
		if got := SanitizeText(in); got != want {
			t.Errorf("SanitizeText(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSanitizeOptionalPreservesNil(t *testing.T) {
	// A missing description must stay NULL, not become an empty string —
	// the two mean different things in the repositories table.
	if got := SanitizeOptional(nil); got != nil {
		t.Errorf("SanitizeOptional(nil) = %v, want nil", got)
	}

	in := "desc\x00with nul"
	got := SanitizeOptional(&in)
	if got == nil {
		t.Fatal("SanitizeOptional returned nil for a non-nil input")
	}
	if *got != "descwith nul" {
		t.Errorf("SanitizeOptional = %q, want %q", *got, "descwith nul")
	}
	if in != "desc\x00with nul" {
		t.Errorf("SanitizeOptional mutated its input: %q", in)
	}
}

func TestSanitizeSlice(t *testing.T) {
	got := SanitizeSlice([]string{"a\x00b", "clean", ""})
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
