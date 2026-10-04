package server_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/Yeti47/frozenfortress/frozenfortress/api/internal/testutil"
	"github.com/Yeti47/frozenfortress/frozenfortress/api/server"
	"github.com/gin-gonic/gin"
)

var securityHeaders = map[string]string{
	"Cache-Control":          "no-store",
	"Pragma":                 "no-cache",
	"X-Content-Type-Options": "nosniff",
	"X-Frame-Options":        "DENY",
	"Referrer-Policy":        "no-referrer",
}

func assertSecurityHeaders(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	for name, want := range securityHeaders {
		if got := rec.Header().Get(name); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
}

func TestSecurityHeadersOnEveryResponse(t *testing.T) {
	ts := testutil.NewTestServer(t, testutil.ServerOptions{})
	for _, path := range []string{"/api/system/health", "/api/system/info", "/api/openapi.json", "/api/does-not-exist"} {
		t.Run(path, func(t *testing.T) {
			assertSecurityHeaders(t, ts.Get(path))
		})
	}
}

func TestUnknownRouteIsProblemJSON(t *testing.T) {
	rec := testutil.NewTestServer(t, testutil.ServerOptions{}).Get("/api/nope")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Fatalf("content type = %q", ct)
	}
}

func TestWrongMethodIsProblemJSON(t *testing.T) {
	rec := testutil.NewTestServer(t, testutil.ServerOptions{}).Do(http.MethodPost, "/api/system/health", nil)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Fatalf("content type = %q", ct)
	}
	assertSecurityHeaders(t, rec)
}

func TestDocsOnlyWhenEnabled(t *testing.T) {
	off := testutil.NewTestServer(t, testutil.ServerOptions{})
	if code := off.Get("/api/docs").Code; code != http.StatusNotFound {
		t.Fatalf("docs disabled: status = %d", code)
	}
	on := testutil.NewTestServer(t, testutil.ServerOptions{Options: server.Options{DocsEnabled: true}})
	if code := on.Get("/api/docs").Code; code != http.StatusOK {
		t.Fatalf("docs enabled: status = %d", code)
	}
}

func TestInvalidTrustedProxies(t *testing.T) {
	if _, _, err := server.NewRouter(server.Deps{}, server.Options{TrustedProxies: []string{"not-a-cidr"}}); err == nil {
		t.Fatal("expected an error")
	}
}

func TestTrustedProxiesControlClientIP(t *testing.T) {
	logs := &captureLogger{}
	build := func(proxies []string) *gin.Engine {
		gin.SetMode(gin.TestMode)
		r, _, err := server.NewRouter(server.Deps{Logger: logs}, server.Options{TrustedProxies: proxies})
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	send := func(r *gin.Engine) {
		req := httptest.NewRequest(http.MethodGet, "/api/system/health", nil)
		req.RemoteAddr = "10.0.0.5:1234"
		req.Header.Set("X-Forwarded-For", "203.0.113.9")
		r.ServeHTTP(httptest.NewRecorder(), req)
	}

	send(build(nil))
	if !strings.Contains(logs.last(), "10.0.0.5") {
		t.Fatalf("untrusted: forwarded header must be ignored, got %s", logs.last())
	}
	send(build([]string{"10.0.0.0/8"}))
	if !strings.Contains(logs.last(), "203.0.113.9") {
		t.Fatalf("trusted: forwarded header must be honoured, got %s", logs.last())
	}
}

func TestRequestLoggerOmitsCookiesBodiesAndQuery(t *testing.T) {
	logs := &captureLogger{}
	ts := testutil.NewTestServer(t, testutil.ServerOptions{Deps: server.Deps{Logger: logs}})

	req := httptest.NewRequest(http.MethodGet, "/api/system/health?token=hunter2", nil)
	req.Header.Set("Cookie", "frozenfortress_session=SESSIONSECRET")
	req.Header.Set("Authorization", "Bearer BEARERSECRET")
	ts.Router.ServeHTTP(httptest.NewRecorder(), req)

	line := logs.last()
	if !strings.Contains(line, "/api/system/health") || !strings.Contains(line, "200") {
		t.Fatalf("request not logged: %q", line)
	}
	for _, secret := range []string{"SESSIONSECRET", "BEARERSECRET", "hunter2"} {
		if strings.Contains(line, secret) {
			t.Fatalf("log line leaks %q: %s", secret, line)
		}
	}
}

func TestPanicIsRecoveredAsProblemJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logs := &captureLogger{}
	router, _, err := server.NewRouter(server.Deps{Logger: logs}, server.Options{})
	if err != nil {
		t.Fatal(err)
	}
	router.GET("/api/boom", func(c *gin.Context) { panic("kaboom secret") })

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/boom", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Fatalf("content type = %q", ct)
	}
	if strings.Contains(rec.Body.String(), "kaboom") {
		t.Fatalf("panic value leaked: %s", rec.Body)
	}
	assertSecurityHeaders(t, rec)
}

type captureLogger struct {
	mu    sync.Mutex
	lines []string
}

func (l *captureLogger) record(msg string, args []any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	var b strings.Builder
	b.WriteString(msg)
	for _, a := range args {
		b.WriteString(" ")
		b.WriteString(strings.TrimSpace(strings.ReplaceAll(fmtAny(a), "\n", " ")))
	}
	l.lines = append(l.lines, b.String())
}

func (l *captureLogger) last() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.lines) == 0 {
		return ""
	}
	return l.lines[len(l.lines)-1]
}

func (l *captureLogger) Info(msg string, args ...any)  { l.record(msg, args) }
func (l *captureLogger) Warn(msg string, args ...any)  { l.record(msg, args) }
func (l *captureLogger) Error(msg string, args ...any) { l.record(msg, args) }
func (l *captureLogger) Debug(msg string, args ...any) { l.record(msg, args) }

func fmtAny(a any) string { return fmt.Sprint(a) }
