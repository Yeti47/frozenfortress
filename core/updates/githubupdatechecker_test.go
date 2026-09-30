package updates

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

const releasesFixture = `[
  {"tag_name": "android-v0.3.0", "html_url": "https://example.test/android-v0.3.0", "draft": false, "prerelease": false},
  {"tag_name": "v2.0.0-beta1", "html_url": "https://example.test/v2.0.0-beta1", "draft": false, "prerelease": true},
  {"tag_name": "v1.10.0", "html_url": "https://example.test/v1.10.0", "draft": true, "prerelease": false},
  {"tag_name": "v1.4.1", "html_url": "https://example.test/v1.4.1", "draft": false, "prerelease": false, "published_at": "2026-09-30T21:32:10Z"},
  {"tag_name": "v1.9.0", "html_url": "https://example.test/v1.9.0", "draft": false, "prerelease": true},
  {"tag_name": "v1.4.0", "html_url": "https://example.test/v1.4.0", "draft": false, "prerelease": false},
  {"tag_name": "v1.3.10", "html_url": "https://example.test/v1.3.10", "draft": false, "prerelease": false}
]`

func newFixtureServer(t *testing.T, status int, body string) (*httptest.Server, *int32) {
	t.Helper()
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		if r.Header.Get("User-Agent") == "" {
			t.Errorf("expected a User-Agent header")
		}
		w.WriteHeader(status)
		w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

func TestCheckFindsNewestStableRelease(t *testing.T) {
	srv, _ := newFixtureServer(t, http.StatusOK, releasesFixture)
	checker := NewGitHubUpdateChecker(srv.URL, "1.3.0", nil)

	got, err := checker.Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Fatalf("expected an update, got nil")
	}
	if got.Version != "1.4.1" || got.URL != "https://example.test/v1.4.1" {
		t.Errorf("unexpected release: %+v", got)
	}
	if got.PublishedAt.IsZero() {
		t.Errorf("expected PublishedAt to be set")
	}
	if checker.Latest() != got {
		t.Errorf("Latest() should return the cached result")
	}
}

func TestCheckUpToDate(t *testing.T) {
	for _, current := range []string{"1.4.1", "v1.4.1", "1.5.0"} {
		srv, _ := newFixtureServer(t, http.StatusOK, releasesFixture)
		checker := NewGitHubUpdateChecker(srv.URL, current, nil)

		got, err := checker.Check(context.Background())
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", current, err)
		}
		if got != nil || checker.Latest() != nil {
			t.Errorf("%s: expected no update, got %+v", current, got)
		}
	}
}

func TestCheckSkipsNonReleaseVersion(t *testing.T) {
	for _, current := range []string{"dev", "", "1.4.1-rc1"} {
		srv, hits := newFixtureServer(t, http.StatusOK, releasesFixture)
		checker := NewGitHubUpdateChecker(srv.URL, current, nil)

		got, err := checker.Check(context.Background())
		if err != nil || got != nil {
			t.Errorf("%q: expected (nil, nil), got (%+v, %v)", current, got, err)
		}
		if atomic.LoadInt32(hits) != 0 {
			t.Errorf("%q: expected no request to be made", current)
		}
	}
}

func TestCheckHTTPErrorKeepsPreviousResult(t *testing.T) {
	srv, _ := newFixtureServer(t, http.StatusOK, releasesFixture)
	checker := NewGitHubUpdateChecker(srv.URL, "1.0.0", nil)
	if _, err := checker.Check(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	failing, _ := newFixtureServer(t, http.StatusForbidden, `{"message":"rate limited"}`)
	checker.releasesURL = failing.URL
	if _, err := checker.Check(context.Background()); err == nil {
		t.Fatalf("expected an error for HTTP 403")
	}
	if checker.Latest() == nil || checker.Latest().Version != "1.4.1" {
		t.Errorf("expected previous result to be kept, got %+v", checker.Latest())
	}
}

func TestCheckInvalidJSON(t *testing.T) {
	srv, _ := newFixtureServer(t, http.StatusOK, `not json`)
	checker := NewGitHubUpdateChecker(srv.URL, "1.0.0", nil)
	if _, err := checker.Check(context.Background()); err == nil {
		t.Fatalf("expected a decode error")
	}
}

func TestCheckNetworkError(t *testing.T) {
	srv, _ := newFixtureServer(t, http.StatusOK, releasesFixture)
	url := srv.URL
	srv.Close()

	checker := NewGitHubUpdateChecker(url, "1.0.0", nil)
	if _, err := checker.Check(context.Background()); err == nil {
		t.Fatalf("expected a network error")
	}
}

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"v1.10.0", "v1.9.9", 1},
		{"v1.3.10", "v1.3.9", 1},
		{"v2.0.0", "v1.99.99", 1},
		{"v1.4.1", "v1.4.1", 0},
		{"v0.9.0", "v1.0.0", -1},
	}
	for _, c := range cases {
		a, okA := parseVersion(c.a)
		b, okB := parseVersion(c.b)
		if !okA || !okB {
			t.Fatalf("failed to parse %s or %s", c.a, c.b)
		}
		got := compareVersions(a, b)
		if (got > 0) != (c.want > 0) || (got < 0) != (c.want < 0) {
			t.Errorf("compare(%s, %s) = %d, want sign of %d", c.a, c.b, got, c.want)
		}
	}
}
