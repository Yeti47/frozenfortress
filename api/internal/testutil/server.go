package testutil

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Yeti47/frozenfortress/frozenfortress/api/server"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
	"github.com/gin-gonic/gin"
)

// TestServer is a router with request helpers.
type TestServer struct {
	Router *gin.Engine
}

// NewTestServer builds the production router (server.NewRouter, default options) around the
// given handlers, which the test constructs with whatever fakes it needs.
func NewTestServer(t testing.TB, handlers ...server.Registrar) *TestServer {
	t.Helper()
	gin.SetMode(gin.TestMode)

	router, _, err := server.NewRouter(ccc.NopLogger, server.Options{}, handlers...)
	if err != nil {
		t.Fatalf("failed to build router: %v", err)
	}
	return &TestServer{Router: router}
}

// Do sends a request with an optional JSON body and returns the recorded response.
func (s *TestServer) Do(method, path string, body any) *httptest.ResponseRecorder {
	var reader *bytes.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			panic(err)
		}
		reader = bytes.NewReader(data)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)
	return rec
}

// Get sends a GET request.
func (s *TestServer) Get(path string) *httptest.ResponseRecorder {
	return s.Do(http.MethodGet, path, nil)
}
