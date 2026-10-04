package testutil

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Yeti47/frozenfortress/frozenfortress/api/server"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/encryption"
	"github.com/gin-gonic/gin"
)

// ServerOptions override the default fakes of NewTestServer. Zero fields keep the default.
type ServerOptions struct {
	Deps    server.Deps
	Options server.Options
}

// TestServer is the real router, built by server.NewRouter, with injectable services.
type TestServer struct {
	Router *gin.Engine
}

// NewTestServer builds the production router with fakes for every service the options leave unset.
func NewTestServer(t testing.TB, opts ServerOptions) *TestServer {
	t.Helper()
	gin.SetMode(gin.TestMode)

	deps := opts.Deps
	if deps.Logger == nil {
		deps.Logger = ccc.NopLogger
	}
	if deps.DB == nil {
		deps.DB = &FakePinger{}
	}
	if deps.UpdateChecker == nil {
		deps.UpdateChecker = &FakeUpdateChecker{}
	}
	if deps.SignInManager == nil {
		deps.SignInManager = NewAuthedSignInManager("user-1")
	}
	if deps.MekStore == nil {
		deps.MekStore = NewFakeMekStore()
	}
	if deps.EncryptionService == nil {
		deps.EncryptionService = encryption.NewDefaultEncryptionService()
	}

	router, _, err := server.NewRouter(deps, opts.Options)
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
