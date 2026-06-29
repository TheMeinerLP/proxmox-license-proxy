package release

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestIsNewer(t *testing.T) {
	cases := []struct {
		current, latest string
		want            bool
	}{
		{"1.2.0", "v1.3.0", true},
		{"v1.2.0", "1.2.1", true},
		{"1.2.0", "1.2.0", false},
		{"1.3.0", "1.2.9", false},
		{"1.2.0", "1.10.0", true},   // numeric, not lexical
		{"dev", "v1.0.0", true},     // unknown local build -> any release is newer
		{"", "v1.0.0", true},        // empty -> treat as outdated
		{"1.2.0", "garbage", false}, // unparseable release -> cannot tell
		{"1.2.0-rc1", "1.2.0", false},
		{"1.2.0", "1.2.1-rc1", true}, // suffix ignored, patch still newer
	}
	for _, c := range cases {
		if got := IsNewer(c.current, c.latest); got != c.want {
			t.Errorf("IsNewer(%q, %q) = %v, want %v", c.current, c.latest, got, c.want)
		}
	}
}

func TestLatest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("missing User-Agent header")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tag_name":"v1.4.0","html_url":"https://example.test/releases/v1.4.0"}`))
	}))
	defer srv.Close()

	c := &Checker{URL: srv.URL, Client: &http.Client{Timeout: 2 * time.Second}}
	info, err := c.Latest(context.Background())
	if err != nil {
		t.Fatalf("Latest: %v", err)
	}
	if info.Tag != "v1.4.0" {
		t.Errorf("Tag = %q, want v1.4.0", info.Tag)
	}
	if info.URL != "https://example.test/releases/v1.4.0" {
		t.Errorf("URL = %q", info.URL)
	}
}

func TestLatestHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden) // e.g. rate-limited
	}))
	defer srv.Close()

	c := &Checker{URL: srv.URL, Client: &http.Client{Timeout: 2 * time.Second}}
	if _, err := c.Latest(context.Background()); err == nil {
		t.Fatal("expected an error on non-200 response")
	}
}
