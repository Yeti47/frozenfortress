package system_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Yeti47/frozenfortress/frozenfortress/api/internal/testutil"
	"github.com/Yeti47/frozenfortress/frozenfortress/api/server"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/updates"
	"github.com/gin-gonic/gin"
)

func TestHealth_OK(t *testing.T) {
	ts := testutil.NewTestServer(t, testutil.ServerOptions{})
	rec := ts.Get("/api/system/health")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d (%s)", rec.Code, rec.Body)
	}
	if got := strings.TrimSpace(rec.Body.String()); got != `{"status":"ok"}` {
		t.Fatalf("body = %s", got)
	}
}

func TestHealth_DatabaseDown(t *testing.T) {
	ts := testutil.NewTestServer(t, testutil.ServerOptions{
		Deps: server.Deps{DB: &testutil.FakePinger{Err: errors.New("db gone")}},
	})
	rec := ts.Get("/api/system/health")

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Fatalf("content type = %q", ct)
	}
	if strings.Contains(rec.Body.String(), "db gone") {
		t.Fatalf("internal error leaked: %s", rec.Body)
	}
}

func TestInfo_NoNewerRelease(t *testing.T) {
	ts := testutil.NewTestServer(t, testutil.ServerOptions{})
	rec := ts.Get("/api/system/info")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d (%s)", rec.Code, rec.Body)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	if string(raw["version"]) != `"`+ccc.AppVersion+`"` {
		t.Fatalf("version = %s", raw["version"])
	}
	if string(raw["latestRelease"]) != "null" {
		t.Fatalf("latestRelease = %s, want null", raw["latestRelease"])
	}
}

func TestInfo_NewerRelease(t *testing.T) {
	published := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	ts := testutil.NewTestServer(t, testutil.ServerOptions{
		Deps: server.Deps{UpdateChecker: &testutil.FakeUpdateChecker{Release: &updates.ReleaseInfo{
			Version: "9.9.9", URL: "https://example.com/r", PublishedAt: published,
		}}},
	})
	rec := ts.Get("/api/system/info")

	var body struct {
		Version       string `json:"version"`
		LatestRelease *struct {
			Version     string    `json:"version"`
			URL         string    `json:"url"`
			PublishedAt time.Time `json:"publishedAt"`
		} `json:"latestRelease"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.LatestRelease == nil || body.LatestRelease.Version != "9.9.9" ||
		body.LatestRelease.URL != "https://example.com/r" || !body.LatestRelease.PublishedAt.Equal(published) {
		t.Fatalf("unexpected body: %s", rec.Body)
	}
}

func TestNilServices(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router, _, err := server.NewRouter(server.Deps{}, server.Options{})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/api/system/health", "/api/system/info"} {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status = %d", path, rec.Code)
		}
	}
}
