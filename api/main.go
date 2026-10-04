package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Yeti47/frozenfortress/frozenfortress/api/server"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
)

// envDocsEnabled turns on the interactive API docs UI. Never set it in production.
const envDocsEnabled = "FF_API_DOCS_ENABLED"

const shutdownTimeout = 30 * time.Second

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	config := ccc.LoadConfigFromEnv()

	db, err := ccc.SetupDatabase(config)
	if err != nil {
		return fmt.Errorf("failed to setup database: %w", err)
	}
	defer db.Close()

	svc := configureServices(config, db)

	router, _, err := server.NewRouter(server.Deps{
		Logger:            svc.Logger,
		DB:                db,
		UpdateChecker:     svc.UpdateChecker,
		SignInManager:     svc.SignInManager,
		MekStore:          svc.MekStore,
		EncryptionService: svc.EncryptionService,
	}, server.Options{
		TrustedProxies: config.TrustedProxies,
		DocsEnabled:    os.Getenv(envDocsEnabled) == "true",
	})
	if err != nil {
		return err
	}

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", config.APIPort),
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	ln, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", srv.Addr, err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// The deferred db.Close runs last, after serve has stopped the workers and drained requests.
	return serve(ctx, srv, ln, svc.Logger, shutdownTimeout, svc.BackupWorker, svc.UpdateCheckWorker)
}

// worker is a background job with a start/stop lifecycle.
type worker interface {
	Start()
	Stop()
}

// serve starts the workers and serves srv on ln until ctx is cancelled or the server fails.
// It then shuts down in a fixed order: stop the workers, drain in-flight requests (up to
// timeout), and only then return, so the caller can close the DB.
func serve(ctx context.Context, srv *http.Server, ln net.Listener, logger ccc.Logger, timeout time.Duration, workers ...worker) error {
	for _, w := range workers {
		w.Start()
	}

	serveErr := make(chan error, 1)
	go func() {
		logger.Info("API listening", "addr", ln.Addr().String())
		serveErr <- srv.Serve(ln)
	}()

	var result error
	select {
	case err := <-serveErr:
		if !errors.Is(err, http.ErrServerClosed) {
			result = err
		}
	case <-ctx.Done():
		logger.Info("Shutdown signal received")
	}

	logger.Info("Shutting down background workers...")
	for _, w := range workers {
		w.Stop()
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("Graceful shutdown failed", "error", err)
		srv.Close() // give up on requests still in flight
		if result == nil {
			result = err
		}
	}
	return result
}
