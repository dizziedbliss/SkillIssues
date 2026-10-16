package httpx

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDoTimesOutAStalledUpstream(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { <-release }))
	defer srv.Close()
	defer close(release)

	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	start := time.Now()
	_, err := New(func() time.Duration { return 50 * time.Millisecond }, nil).Do(req)
	if err == nil {
		t.Fatal("expected a timeout error")
	}
	if time.Since(start) > 2*time.Second {
		t.Fatalf("took %s; the timeout was not applied", time.Since(start))
	}
}

func TestDoRefusesWithoutATimeout(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "http://127.0.0.1:1", nil)
	for name, c := range map[string]*Client{
		"nil client": nil,
		"nil func":   New(nil, nil),
		"zero":       New(func() time.Duration { return 0 }, nil),
	} {
		if _, err := c.Do(req); err != ErrNoTimeout {
			t.Errorf("%s: err = %v, want ErrNoTimeout", name, err)
		}
	}
}

func TestTimeoutIsReadPerCall(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(150 * time.Millisecond)
	}))
	defer srv.Close()
	d := 20 * time.Millisecond
	c := New(func() time.Duration { return d }, nil)

	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	if _, err := c.Do(req); err == nil {
		t.Fatal("first call should time out")
	}
	d = 5 * time.Second
	req, _ = http.NewRequest(http.MethodGet, srv.URL, nil)
	resp, err := c.Do(req)
	if err != nil {
		t.Fatalf("second call after raising the timeout: %v", err)
	}
	resp.Body.Close()
}
