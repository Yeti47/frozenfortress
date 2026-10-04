// Package system holds the unauthenticated system endpoints: health and info.
// It is the pattern for every resource package: a Handler object whose dependencies are
// injected into NewHandler, a Register method that only takes the huma.API, input/output
// types with explicit json tags, and errors returned through problem.Map.
package system

import (
	"context"
	"net/http"
	"time"

	"github.com/Yeti47/frozenfortress/frozenfortress/api/internal/problem"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/updates"
	"github.com/danielgtaylor/huma/v2"
)

// Pinger is the part of *sql.DB the health check needs.
type Pinger interface {
	PingContext(ctx context.Context) error
}

// Handler serves the system endpoints.
type Handler struct {
	logger        ccc.Logger
	db            Pinger
	updateChecker updates.UpdateChecker
}

// NewHandler creates a Handler.
func NewHandler(logger ccc.Logger, db Pinger, updateChecker updates.UpdateChecker) *Handler {
	return &Handler{logger: logger, db: db, updateChecker: updateChecker}
}

// healthBody is the response of GET /api/system/health.
type healthBody struct {
	Status string `json:"status" enum:"ok" doc:"Always \"ok\" when the API is healthy."`
}

// healthOutput wraps healthBody for Huma.
type healthOutput struct {
	Body healthBody
}

// latestRelease describes a newer published release.
type latestRelease struct {
	Version     string    `json:"version" doc:"Version without a leading \"v\"." example:"1.4.1"`
	URL         string    `json:"url" doc:"Link to the release page."`
	PublishedAt time.Time `json:"publishedAt"`
}

// infoBody is the response of GET /api/system/info.
type infoBody struct {
	Version       string         `json:"version" doc:"Version of the running API." example:"1.4.0"`
	LatestRelease *latestRelease `json:"latestRelease,omitempty" doc:"The newest release. Absent when the running version is up to date, or when the update check is disabled or has not succeeded yet."`
}

// infoOutput wraps infoBody for Huma.
type infoOutput struct {
	Body infoBody
}

// Register adds the system operations to api.
func (h *Handler) Register(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "getHealth",
		Method:      http.MethodGet,
		Path:        "/api/system/health",
		Summary:     "Liveness and database check",
		Description: "Used by the container healthcheck. Pings the database only; never touches Redis or Ollama.",
		Tags:        []string{"System"},
		Errors:      []int{http.StatusServiceUnavailable},
	}, h.getHealth)

	huma.Register(api, huma.Operation{
		OperationID: "getInfo",
		Method:      http.MethodGet,
		Path:        "/api/system/info",
		Summary:     "Version and update information",
		Tags:        []string{"System"},
	}, h.getInfo)
}

func (h *Handler) getHealth(ctx context.Context, _ *struct{}) (*healthOutput, error) {
	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := h.db.PingContext(pingCtx); err != nil {
		return nil, problem.Map(h.logger, &ccc.ApiError{
			StatusCode:       http.StatusServiceUnavailable,
			Code:             ccc.ErrCodeDatabaseError,
			UserMessage:      "Database unavailable",
			TechnicalMessage: "health check database ping failed",
			Cause:            err,
		})
	}
	return &healthOutput{Body: healthBody{Status: "ok"}}, nil
}

func (h *Handler) getInfo(ctx context.Context, _ *struct{}) (*infoOutput, error) {
	body := infoBody{Version: ccc.AppVersion}
	if release := h.updateChecker.Latest(); release != nil {
		body.LatestRelease = &latestRelease{
			Version:     release.Version,
			URL:         release.URL,
			PublishedAt: release.PublishedAt,
		}
	}
	return &infoOutput{Body: body}, nil
}
