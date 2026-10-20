package theme

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The fixtures are shared with packages/theme, so the server and the web app
// accept and refuse exactly the same themes.
const fixtures = "../../../packages/theme/fixtures"

func eachFixture(t *testing.T, dir string, fn func(t *testing.T, src string)) {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(fixtures, dir, "*"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no fixtures in %s (%v)", dir, err)
	}
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		t.Run(filepath.Base(f), func(t *testing.T) { fn(t, string(src)) })
	}
}

func TestTOMLFixtures(t *testing.T) {
	eachFixture(t, "toml/valid", func(t *testing.T, src string) {
		if _, err := ParseTOML(src); err != nil {
			t.Errorf("refused: %v", err)
		}
	})
	eachFixture(t, "toml/invalid", func(t *testing.T, src string) {
		_, err := ParseTOML(src)
		if !errors.Is(err, ErrInvalid) {
			t.Errorf("accepted, or refused without ErrInvalid: %v", err)
		}
	})
}

func TestCSSFixtures(t *testing.T) {
	eachFixture(t, "css/valid", func(t *testing.T, src string) {
		if err := CheckCSS(src); err != nil {
			t.Errorf("refused: %v", err)
		}
	})
	eachFixture(t, "css/invalid", func(t *testing.T, src string) {
		if err := CheckCSS(src); !errors.Is(err, ErrInvalid) {
			t.Errorf("accepted: %q", src)
		}
	})
}

// Every problem is reported at once, by path.
func TestProblemsNameEveryKey(t *testing.T) {
	_, err := ParseTOML("[colors]\naccent = 1\nnope = \"#fff\"\n[x]\ny = 1\n")
	for _, want := range []string{"colors.accent", "colors.nope", "[x]"} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("error %v does not mention %s", err, want)
		}
	}
}

func TestName(t *testing.T) {
	tok, err := ParseTOML("[meta]\nname = \"  Paper  \"\n")
	if err != nil || tok.Name() != "Paper" {
		t.Errorf("Name() = %q, %v", tok.Name(), err)
	}
}
