package main

import (
	"context"
	"errors"
	"fmt"
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

	svc.BackupWorker.Start()
	svc.UpdateCheckWorker.Start()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serveErr := make(chan error, 1)
	go func() {
		svc.Logger.Info("API listening", "addr", srv.Addr)
		serveErr <- srv.ListenAndServe()
	}()

	var result error
	select {
	case err := <-serveErr:
		if !errors.Is(err, http.ErrServerClosed) {
			result = err
		}
	case <-ctx.Done():
		svc.Logger.Info("Shutdown signal received")
	}

	// 1. stop the workers, 2. drain in-flight requests, 3. close the DB (deferred).
	svc.Logger.Info("Shutting down background workers...")
	svc.BackupWorker.Stop()
	svc.UpdateCheckWorker.Stop()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		svc.Logger.Error("Graceful shutdown failed", "error", err)
		if result == nil {
			result = err
		}
	}
	return result
}
