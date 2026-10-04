// Package system holds the unauthenticated system endpoints: health and info.
// It is the pattern for every resource package: a Register function, input/output
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

// Deps are the services the system endpoints use.
type Deps struct {
	Logger        ccc.Logger
	DB            Pinger
	UpdateChecker updates.UpdateChecker
}

// HealthBody is the response of GET /api/system/health.
type HealthBody struct {
	Status string `json:"status" enum:"ok" doc:"Always \"ok\" when the API is healthy."`
}

// HealthOutput wraps HealthBody for Huma.
type HealthOutput struct {
	Body HealthBody
}

// LatestRelease describes a newer published release.
type LatestRelease struct {
	Version     string    `json:"version" doc:"Version without a leading \"v\"." example:"1.4.1"`
	URL         string    `json:"url" doc:"Link to the release page."`
	PublishedAt time.Time `json:"publishedAt"`
}

// InfoBody is the response of GET /api/system/info.
type InfoBody struct {
	Version       string         `json:"version" doc:"Version of the running API." example:"1.4.0"`
	LatestRelease *LatestRelease `json:"latestRelease" doc:"The newest release, only set when it is newer than the running version."`
}

// TransformSchema makes latestRelease nullable in the spec: Huma cannot express that for
// a pointer to a struct through a struct tag.
func (InfoBody) TransformSchema(r huma.Registry, s *huma.Schema) *huma.Schema {
	if prop, ok := s.Properties["latestRelease"]; ok {
		s.Properties["latestRelease"] = &huma.Schema{
			Description: prop.Description,
			OneOf:       []*huma.Schema{{Ref: prop.Ref}, {Type: "null"}},
		}
	}
	return s
}

// InfoOutput wraps InfoBody for Huma.
type InfoOutput struct {
	Body InfoBody
}

// Register adds the system operations to api.
func Register(api huma.API, deps Deps) {
	huma.Register(api, huma.Operation{
		OperationID: "getHealth",
		Method:      http.MethodGet,
		Path:        "/api/system/health",
		Summary:     "Liveness and database check",
		Description: "Used by the container healthcheck. Pings the database only; never touches Redis or Ollama.",
		Tags:        []string{"System"},
		Errors:      []int{http.StatusServiceUnavailable},
	}, func(ctx context.Context, _ *struct{}) (*HealthOutput, error) {
		if deps.DB != nil {
			pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
			defer cancel()
			if err := deps.DB.PingContext(pingCtx); err != nil {
				return nil, problem.Map(deps.Logger, &ccc.ApiError{
					StatusCode:       http.StatusServiceUnavailable,
					Code:             ccc.ErrCodeDatabaseError,
					UserMessage:      "Database unavailable",
					TechnicalMessage: "health check database ping failed",
					Cause:            err,
				})
			}
		}
		return &HealthOutput{Body: HealthBody{Status: "ok"}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "getInfo",
		Method:      http.MethodGet,
		Path:        "/api/system/info",
		Summary:     "Version and update information",
		Tags:        []string{"System"},
	}, func(ctx context.Context, _ *struct{}) (*InfoOutput, error) {
		body := InfoBody{Version: ccc.AppVersion}
		if deps.UpdateChecker != nil {
			if release := deps.UpdateChecker.Latest(); release != nil {
				body.LatestRelease = &LatestRelease{
					Version:     release.Version,
					URL:         release.URL,
					PublishedAt: release.PublishedAt,
				}
			}
		}
		return &InfoOutput{Body: body}, nil
	})
}
