// Package server builds the HTTP router and the Huma API. It is used by main and
// by the tests, so both exercise the same wiring.
package server

import (
	"fmt"
	"net/http"
	"time"

	"github.com/Yeti47/frozenfortress/frozenfortress/api/internal/problem"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humagin"
	"github.com/gin-gonic/gin"
)

// Registrar adds a group of operations to the API. Every handler object implements it; its
// dependencies are injected into its own constructor, not into the router.
type Registrar interface {
	Register(api huma.API)
}

// Options are the settings that do not come from services.
type Options struct {
	// TrustedProxies are the CIDRs/IPs whose X-Forwarded-* headers are trusted. Empty trusts none.
	TrustedProxies []string
	// DocsEnabled serves the interactive API docs UI at /api/docs. Keep it off in production.
	DocsEnabled bool
}

// NewRouter builds the gin engine and the Huma API on top of it.
//
// NOTE for upload operations: Huma's default MaxBodyBytes is 1 MB. Operations that accept
// file uploads must raise it explicitly (huma.Operation.MaxBodyBytes).
func NewRouter(logger ccc.Logger, opts Options, handlers ...Registrar) (*gin.Engine, huma.API, error) {
	if logger == nil {
		logger = ccc.NopLogger
	}

	router := gin.New()
	// With no trusted proxies configured gin must not honour forwarding headers at all.
	if err := router.SetTrustedProxies(opts.TrustedProxies); err != nil {
		return nil, nil, fmt.Errorf("invalid trusted proxies: %w", err)
	}

	router.Use(
		gin.CustomRecoveryWithWriter(nil, func(c *gin.Context, recovered any) {
			logger.Error("Panic while handling request", "path", c.Request.URL.Path, "panic", fmt.Sprint(recovered))
			writeProblem(c, problem.Map(logger, fmt.Errorf("panic: %v", recovered)))
		}),
		securityHeaders(),
		requestLogger(logger),
	)
	// TODO(YETI-83): register the auth session and CSRF (double-submit XSRF) middleware here,
	// after the security headers and before the routes.

	router.NoRoute(func(c *gin.Context) {
		writeProblem(c, huma.NewError(http.StatusNotFound, "Not found"))
	})
	router.HandleMethodNotAllowed = true
	router.NoMethod(func(c *gin.Context) {
		writeProblem(c, huma.NewError(http.StatusMethodNotAllowed, "Method not allowed"))
	})

	api := humagin.New(router, apiConfig(opts))
	for _, h := range handlers {
		h.Register(api)
	}

	return router, api, nil
}

func apiConfig(opts Options) huma.Config {
	config := huma.DefaultConfig("FrozenFortress API", ccc.AppVersion)
	config.OpenAPIPath = "/api/openapi"
	config.SchemasPath = "/api/schemas"
	config.DocsPath = ""
	if opts.DocsEnabled {
		config.DocsPath = "/api/docs"
	}
	// Keep responses plain: no $schema property or Link header on every body.
	config.CreateHooks = nil
	return config
}

// writeProblem writes err as application/problem+json for responses produced outside Huma.
func writeProblem(c *gin.Context, err huma.StatusError) {
	c.Header("Content-Type", "application/problem+json")
	c.JSON(err.GetStatus(), err)
	c.Abort()
}

// securityHeaders sets the response headers every /api response must carry.
func securityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("Cache-Control", "no-store")
		h.Set("Pragma", "no-cache")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		c.Next()
	}
}

// requestLogger logs one structured line per request. It deliberately logs neither
// headers (Cookie, Authorization) nor bodies, and not the query string.
func requestLogger(logger ccc.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		logger.Info("Request",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"durationMs", time.Since(start).Milliseconds(),
			"clientIp", c.ClientIP(),
		)
	}
}
