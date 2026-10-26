package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// The repository the installer installs from. Only its release files are
// ever downloaded.
const (
	repo = "dizziedbliss/SkillIssues"
	// latestURL is electron-builder's update file on the newest release: the
	// version, the installer's name, its size and its SHA-512. Read through
	// GitHub's latest-release redirect, so no API call and no rate limit.
	latestURL = "https://github.com/" + repo + "/releases/latest/download/latest.yml"
)

// Release is the newest Windows build, as latest.yml describes it.
type Release struct {
	Version string // "1.5.0"
	Path    string // "SkillIssues-Setup-1.5.0.exe"
	SHA512  string // base64, as electron-builder writes it
	Size    int64  // bytes; 0 when the file doesn't say
}

// AssetURL is where the release's installer is downloaded from.
func (r Release) AssetURL() string {
	return "https://github.com/" + repo + "/releases/download/v" + url.PathEscape(r.Version) + "/" + url.PathEscape(r.Path)
}

// ParseLatest reads the parts of latest.yml the installer needs. It is a
// small, strict reader for the one shape electron-builder writes, not a YAML
// parser: top-level "version", "path" and "sha512", and the size of the
// "files" entry whose url is that path.
func ParseLatest(r io.Reader) (Release, error) {
	var rel Release
	var inFiles bool
	var entryURL string
	sizes := map[string]int64{}
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := sc.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		indented := strings.HasPrefix(line, " ")
		if !indented {
			inFiles = false
		}
		key, value, ok := strings.Cut(strings.TrimPrefix(trimmed, "- "), ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = unquote(strings.TrimSpace(value))
		if !indented {
			switch key {
			case "version":
				rel.Version = value
			case "path":
				rel.Path = value
			case "sha512":
				rel.SHA512 = value
			case "files":
				inFiles = true
			}
			continue
		}
		if !inFiles {
			continue
		}
		switch key {
		case "url":
			entryURL = value
		case "size":
			n, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return Release{}, fmt.Errorf("latest.yml: size %q: %w", value, err)
			}
			sizes[entryURL] = n
		}
	}
	if err := sc.Err(); err != nil {
		return Release{}, fmt.Errorf("latest.yml: %w", err)
	}
	if rel.Version == "" || rel.Path == "" || rel.SHA512 == "" {
		return Release{}, errors.New("latest.yml: version, path or sha512 missing")
	}
	if _, err := ParseVersion(rel.Version); err != nil {
		return Release{}, fmt.Errorf("latest.yml: %w", err)
	}
	// The installer's name ends up in a URL and a file path: nothing but a plain file name.
	if strings.ContainsAny(rel.Path, `/\:`) || !strings.HasSuffix(strings.ToLower(rel.Path), ".exe") {
		return Release{}, fmt.Errorf("latest.yml: unexpected installer name %q", rel.Path)
	}
	rel.Size = sizes[rel.Path]
	return rel, nil
}

func unquote(s string) string {
	if len(s) >= 2 && (s[0] == '\'' && s[len(s)-1] == '\'' || s[0] == '"' && s[len(s)-1] == '"') {
		return s[1 : len(s)-1]
	}
	return s
}

// allowedHosts are where GitHub serves release files from: github.com, which
// redirects a download to one of its storage hosts.
var allowedHosts = map[string]bool{
	"github.com":                           true,
	"objects.githubusercontent.com":        true,
	"release-assets.githubusercontent.com": true,
}

// allowed reports whether the installer may fetch u: https, and one of GitHub's release hosts.
func allowed(u *url.URL) bool {
	return u.Scheme == "https" && allowedHosts[strings.ToLower(u.Hostname())] && u.User == nil
}

// newClient is the HTTP client for every request: it follows redirects only
// to GitHub's release hosts, over https, and gives up on a stalled connection.
func newClient() *http.Client {
	return &http.Client{
		Timeout: 0, // a download can take minutes; the context and the transport bound it instead
		Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			TLSHandshakeTimeout:   15 * time.Second,
			ResponseHeaderTimeout: 30 * time.Second,
			IdleConnTimeout:       30 * time.Second,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("too many redirects")
			}
			if !allowed(req.URL) {
				return fmt.Errorf("refusing to follow a redirect to %s", req.URL.Host)
			}
			return nil
		},
	}
}
