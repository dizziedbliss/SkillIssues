package main

import (
	"context"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Progress is a download's state, as the window shows it.
type Progress struct {
	Done        int64   `json:"done"`
	Total       int64   `json:"total"` // 0 when unknown
	BytesPerSec float64 `json:"bytesPerSec"`
}

// progressEvery is how often a download reports its progress to the window.
const progressEvery = 150 * time.Millisecond

// FetchLatest reads the newest release's latest.yml.
func FetchLatest(ctx context.Context, client *http.Client) (Release, error) {
	body, err := get(ctx, client, latestURL)
	if err != nil {
		return Release{}, err
	}
	defer body.Close()
	return ParseLatest(io.LimitReader(body, 64<<10))
}

// Download writes rawURL to dst, reporting progress, and returns the SHA-512
// of what it wrote. want is the size latest.yml gave (0 when unknown); a
// response of a different length is an error.
func Download(ctx context.Context, client *http.Client, rawURL string, want int64, dst io.Writer, report func(Progress)) ([]byte, error) {
	if u, err := url.Parse(rawURL); err != nil || !allowed(u) {
		return nil, fmt.Errorf("refusing to fetch %q", rawURL)
	}
	return downloadFrom(ctx, client, rawURL, want, dst, report)
}

// downloadFrom is Download without the host check, which the caller has made.
func downloadFrom(ctx context.Context, client *http.Client, rawURL string, want int64, dst io.Writer, report func(Progress)) ([]byte, error) {
	body, err := fetch(ctx, client, rawURL)
	if err != nil {
		return nil, err
	}
	defer body.Close()
	total := want
	if cl := body.length; total == 0 && cl > 0 {
		total = cl
	}
	h := sha512.New()
	w := io.MultiWriter(dst, h)
	buf := make([]byte, 64<<10)
	var done int64
	start := time.Now()
	last := time.Time{}
	for {
		n, rerr := body.Read(buf)
		if n > 0 {
			if _, werr := w.Write(buf[:n]); werr != nil {
				return nil, fmt.Errorf("writing the download: %w", werr)
			}
			done += int64(n)
			if now := time.Now(); now.Sub(last) >= progressEvery {
				last = now
				report(Progress{Done: done, Total: total, BytesPerSec: rate(done, start, now)})
			}
		}
		if errors.Is(rerr, io.EOF) {
			break
		}
		if rerr != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return nil, fmt.Errorf("downloading: %w", rerr)
		}
	}
	report(Progress{Done: done, Total: total, BytesPerSec: rate(done, start, time.Now())})
	if want > 0 && done != want {
		return nil, fmt.Errorf("the download is %d bytes; the release says %d", done, want)
	}
	return h.Sum(nil), nil
}

func rate(done int64, start, now time.Time) float64 {
	if s := now.Sub(start).Seconds(); s > 0 {
		return float64(done) / s
	}
	return 0
}

// VerifySHA512 checks sum against latest.yml's base64 digest.
func VerifySHA512(sum []byte, want string) error {
	expected, err := base64.StdEncoding.DecodeString(want)
	if err != nil || len(expected) != sha512.Size {
		return errors.New("the release's checksum is malformed")
	}
	if subtle.ConstantTimeCompare(sum, expected) != 1 {
		return errors.New("the download doesn't match the release's checksum")
	}
	return nil
}

type response struct {
	io.ReadCloser
	length int64
}

// get fetches an allowed https URL and returns its body, or an error naming the status.
func get(ctx context.Context, client *http.Client, rawURL string) (*response, error) {
	if u, err := url.Parse(rawURL); err != nil || !allowed(u) {
		return nil, fmt.Errorf("refusing to fetch %q", rawURL)
	}
	return fetch(ctx, client, rawURL)
}

// fetch is get without the host check.
func fetch(ctx context.Context, client *http.Client, rawURL string) (*response, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "SkillIssues-Installer")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("GitHub answered %s for %s", resp.Status, u.Path)
	}
	return &response{ReadCloser: resp.Body, length: resp.ContentLength}, nil
}
