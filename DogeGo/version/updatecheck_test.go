// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package version

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSemverCompare(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"0.1.0", "0.1.0", 0},
		{"0.2.0", "0.1.0", 1},
		{"0.1.0", "0.2.0", -1},
		{"v1.0.0", "0.9.9", 1},
		{"0.1.0-beta", "0.1.0", 0},
		{"0.1.1", "0.1.0-beta", 1},
	}
	for _, tc := range cases {
		got := SemverCompare(tc.a, tc.b)
		if got != tc.want {
			t.Fatalf("SemverCompare(%q,%q)=%d want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestUpdateCheckerDismissAndNotify(t *testing.T) {
	dir := t.TempDir()
	c := NewUpdateChecker(dir)
	c.mu.Lock()
	c.status.LatestVersion = "9.9.9"
	c.status.LatestTag = "v9.9.9"
	c.status.ReleaseURL = "https://example.com/release"
	c.recomputeAvailableLocked()
	c.mu.Unlock()

	if !c.Status().Available {
		t.Fatal("expected available")
	}
	if err := c.Dismiss(); err != nil {
		t.Fatal(err)
	}
	if !c.Status().Dismissed {
		t.Fatal("expected dismissed")
	}
	raw, err := os.ReadFile(filepath.Join(dir, updateStateFile))
	if err != nil {
		t.Fatal(err)
	}
	var st updateCheckState
	if err := json.Unmarshal(raw, &st); err != nil {
		t.Fatal(err)
	}
	if st.DismissedVersion != "9.9.9" {
		t.Fatalf("dismissed=%q", st.DismissedVersion)
	}
}

func TestFetchLatestReleasePicksHighest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/repos/qlpqlp/dogego/releases"):
			_ = json.NewEncoder(w).Encode([]ghRelease{{TagName: "v0.1.0", HTMLURL: "https://github.com/qlpqlp/dogego/releases/tag/v0.1.0"}})
		case strings.HasPrefix(r.URL.Path, "/repos/dogeorg/dogego/releases"):
			_ = json.NewEncoder(w).Encode([]ghRelease{
				{TagName: "v0.1.5-beta", HTMLURL: "https://github.com/dogeorg/dogego/releases/tag/v0.1.5-beta", Prerelease: true},
				{TagName: "v0.2.0", HTMLURL: "https://github.com/dogeorg/dogego/releases/tag/v0.2.0"},
			})
		case strings.HasPrefix(r.URL.Path, "/repos/dogecoinfoundation/dogego/releases"):
			http.NotFound(w, r)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	old := DefaultUpdateSources
	DefaultUpdateSources = []UpdateSource{
		{Owner: "qlpqlp", Repo: "dogego", Enabled: true},
		{Owner: "dogeorg", Repo: "dogego", Enabled: true},
		{Owner: "dogecoinfoundation", Repo: "dogego", Enabled: true},
	}
	defer func() { DefaultUpdateSources = old }()

	client := srv.Client()
	client.Transport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		req.URL.Scheme = "http"
		req.URL.Host = srv.Listener.Addr().String()
		return http.DefaultTransport.RoundTrip(req)
	})

	best, sources, err := fetchLatestRelease(context.Background(), client)
	if err != nil {
		t.Fatal(err)
	}
	if len(sources) != 3 {
		t.Fatalf("sources=%v", sources)
	}
	if best.TagName != "v0.2.0" {
		t.Fatalf("tag=%q", best.TagName)
	}
	if best.Source != "https://github.com/dogeorg/dogego" {
		t.Fatalf("source=%q", best.Source)
	}
}

func TestPickReleaseChecksumURL(t *testing.T) {
	rel := ghRelease{
		Assets: []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		}{
			{Name: "dogego-windows-amd64.exe", BrowserDownloadURL: "https://example.com/bin"},
			{Name: "dogego-windows-amd64.exe.sha256", BrowserDownloadURL: "https://example.com/bin.sha256"},
		},
	}
	if got := pickReleaseChecksumURL(rel, "dogego-windows-amd64.exe"); got != "https://example.com/bin.sha256" {
		t.Fatalf("checksum url=%q", got)
	}
}


func TestUpdateCheckerRefreshUsesMock(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]ghRelease{{
			TagName: "v99.0.0",
			HTMLURL: "https://github.com/qlpqlp/dogego/releases/tag/v99.0.0",
			Assets: []struct {
				Name               string `json:"name"`
				BrowserDownloadURL string `json:"browser_download_url"`
			}{{Name: "dogego-linux-amd64.tar.gz", BrowserDownloadURL: "https://example.com/bin"}},
		}})
	}))
	defer srv.Close()

	dir := t.TempDir()
	c := NewUpdateChecker(dir)
	c.client = srv.Client()
	c.client.Transport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		req.URL.Scheme = "http"
		req.URL.Host = srv.Listener.Addr().String()
		return http.DefaultTransport.RoundTrip(req)
	})
	old := DefaultUpdateSources
	DefaultUpdateSources = []UpdateSource{{Owner: "qlpqlp", Repo: "dogego", Enabled: true}}
	defer func() { DefaultUpdateSources = old }()

	c.refresh(context.Background())
	st := c.Status()
	if !st.Available {
		t.Fatalf("available=false latest=%q current=%s", st.LatestVersion, ClientVersion)
	}
	if st.ReleaseURL == "" {
		t.Fatal("missing release url")
	}
	if st.CheckedAt.IsZero() {
		t.Fatal("missing checked at")
	}
}

func TestUpdateCheckerOnAvailable(t *testing.T) {
	dir := t.TempDir()
	c := NewUpdateChecker(dir)
	done := make(chan UpdateStatus, 1)
	c.SetOnAvailable(func(st UpdateStatus) {
		done <- st
	})
	c.mu.Lock()
	c.status.LatestVersion = "9.8.0"
	c.status.LatestTag = "v9.8.0"
	c.recomputeAvailableLocked()
	st, fn, ok := c.takeNotifyCallbackLocked()
	c.mu.Unlock()
	if !ok || fn == nil {
		t.Fatal("expected notify")
	}
	fn(st)
	select {
	case got := <-done:
		if got.LatestVersion != "9.8.0" {
			t.Fatalf("latest=%q", got.LatestVersion)
		}
	default:
		t.Fatal("callback not invoked")
	}
	c.mu.Lock()
	_, _, ok2 := c.takeNotifyCallbackLocked()
	c.mu.Unlock()
	if ok2 {
		t.Fatal("expected single notify per version")
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
