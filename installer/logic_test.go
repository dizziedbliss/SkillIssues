package main

import (
	"bytes"
	"context"
	"crypto/sha512"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// What electron-builder writes for a Windows release.
const sampleLatest = `version: 1.5.0
files:
  - url: SkillIssues-Setup-1.5.0.exe
    sha512: c2hhNTEy
    size: 93458234
path: SkillIssues-Setup-1.5.0.exe
sha512: c2hhNTEy
releaseDate: '2026-10-27T10:00:00.000Z'
`

func TestParseLatest(t *testing.T) {
	rel, err := ParseLatest(strings.NewReader(sampleLatest))
	if err != nil {
		t.Fatal(err)
	}
	want := Release{Version: "1.5.0", Path: "SkillIssues-Setup-1.5.0.exe", SHA512: "c2hhNTEy", Size: 93458234}
	if rel != want {
		t.Fatalf("got %+v, want %+v", rel, want)
	}
	if got := rel.AssetURL(); got != "https://github.com/dizziedbliss/SkillIssues/releases/download/v1.5.0/SkillIssues-Setup-1.5.0.exe" {
		t.Errorf("asset URL %s", got)
	}
}

func TestParseLatestRefusesOddFiles(t *testing.T) {
	cases := map[string]string{
		"missing sha":     "version: 1.5.0\npath: a.exe\n",
		"bad version":     "version: latest\npath: a.exe\nsha512: x\n",
		"path with slash": "version: 1.5.0\npath: ../evil.exe\nsha512: x\n",
		"not an exe":      "version: 1.5.0\npath: setup.zip\nsha512: x\n",
		"bad size":        "version: 1.5.0\nfiles:\n  - url: a.exe\n    size: lots\npath: a.exe\nsha512: x\n",
	}
	for name, body := range cases {
		if _, err := ParseLatest(strings.NewReader(body)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestAllowed(t *testing.T) {
	for raw, want := range map[string]bool{
		"https://github.com/dizziedbliss/SkillIssues/releases/latest/download/latest.yml": true,
		"https://objects.githubusercontent.com/x":                                         true,
		"https://release-assets.githubusercontent.com/x":                                  true,
		"http://github.com/x":               false,
		"https://example.com/x":             false,
		"https://github.com.evil.example/x": false,
		"https://user:pw@github.com/x":      false,
	} {
		u, _ := url.Parse(raw)
		if got := allowed(u); got != want {
			t.Errorf("allowed(%s) = %v, want %v", raw, got, want)
		}
	}
}

func TestVerifySHA512(t *testing.T) {
	data := []byte("the installer")
	sum := sha512.Sum512(data)
	good := base64.StdEncoding.EncodeToString(sum[:])
	if err := VerifySHA512(sum[:], good); err != nil {
		t.Errorf("a matching sum failed: %v", err)
	}
	other := sha512.Sum512([]byte("something else"))
	if err := VerifySHA512(other[:], good); err == nil {
		t.Error("a different sum passed")
	}
	if err := VerifySHA512(sum[:], "not base64!"); err == nil {
		t.Error("a malformed checksum passed")
	}
}

func TestDownloadChecksLengthAndReports(t *testing.T) {
	payload := bytes.Repeat([]byte("x"), 300_000)
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(payload) }))
	defer srv.Close()
	client := srv.Client()
	// The test server isn't one of GitHub's hosts; exercise the body handling directly.
	resp, err := client.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	var buf bytes.Buffer
	var last Progress
	sum, err := downloadFrom(context.Background(), client, srv.URL, int64(len(payload)), &buf, func(p Progress) { last = p })
	if err != nil {
		t.Fatal(err)
	}
	want := sha512.Sum512(payload)
	if !bytes.Equal(sum, want[:]) || buf.Len() != len(payload) {
		t.Fatal("wrong bytes or sum")
	}
	if last.Done != int64(len(payload)) || last.Total != int64(len(payload)) {
		t.Errorf("last progress %+v", last)
	}
	if _, err := downloadFrom(context.Background(), client, srv.URL, int64(len(payload))+1, &bytes.Buffer{}, func(Progress) {}); err == nil {
		t.Error("a short download passed")
	}
}

func TestRefusesHostsOutsideGitHub(t *testing.T) {
	if _, err := Download(context.Background(), newClient(), "https://example.com/setup.exe", 0, &bytes.Buffer{}, func(Progress) {}); err == nil {
		t.Error("downloaded from outside GitHub")
	}
}

func TestCompare(t *testing.T) {
	for _, c := range []struct {
		a, b string
		sign int
	}{
		{"1.5.0", "1.3.0", 1}, {"1.3.0", "1.5.0", -1}, {"v1.5.0", "1.5.0", 0},
		{"2.0.0", "1.99.99", 1}, {"1.5.0-rc.1", "1.5.0", -1}, {"1.10.0", "1.9.0", 1},
	} {
		a, err1 := ParseVersion(c.a)
		b, err2 := ParseVersion(c.b)
		if err1 != nil || err2 != nil {
			t.Fatalf("parse %s / %s: %v %v", c.a, c.b, err1, err2)
		}
		got := Compare(a, b)
		if (got > 0) != (c.sign > 0) || (got < 0) != (c.sign < 0) {
			t.Errorf("Compare(%s, %s) = %d, want sign %d", c.a, c.b, got, c.sign)
		}
	}
	if _, err := ParseVersion("1.5"); err == nil {
		t.Error("accepted 1.5")
	}
}
