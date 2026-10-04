package workers

import (
	"context"
	"time"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/updates"
)

// DefaultUpdateCheckWorker periodically checks for newer releases in the background
type DefaultUpdateCheckWorker struct {
	checker  updates.UpdateChecker
	config   ccc.AppConfig
	logger   ccc.Logger
	interval time.Duration
	ctx      context.Context
	cancel   context.CancelFunc
}

// NewDefaultUpdateCheckWorker creates a new update check worker instance
func NewDefaultUpdateCheckWorker(checker updates.UpdateChecker, config ccc.AppConfig, logger ccc.Logger) *DefaultUpdateCheckWorker {
	if logger == nil {
		logger = ccc.NopLogger
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &DefaultUpdateCheckWorker{
		checker:  checker,
		config:   config,
		logger:   logger,
		interval: 24 * time.Hour,
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Start begins the background worker loop
func (w *DefaultUpdateCheckWorker) Start() {
	if !w.config.UpdateCheckEnabled {
		w.logger.Info("Update check worker disabled via configuration")
		return
	}

	w.logger.Info("Update check worker started")

	go w.run()
}

// Stop gracefully stops the update check worker
func (w *DefaultUpdateCheckWorker) Stop() {
	w.logger.Info("Stopping update check worker")
	w.cancel()
}

// run is the main worker loop that runs in the background
func (w *DefaultUpdateCheckWorker) run() {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	w.performUpdateCheck()

	for {
		select {
		case <-w.ctx.Done():
			w.logger.Info("Update check worker stopped")
			return
		case <-ticker.C:
			w.performUpdateCheck()
		}
	}
}

// performUpdateCheck queries for a newer release and logs the outcome
func (w *DefaultUpdateCheckWorker) performUpdateCheck() {
	release, err := w.checker.Check(w.ctx)
	if err != nil {
		w.logger.Warn("Failed to check for updates", "error", err)
		return
	}

	if release != nil {
		w.logger.Info("A newer version is available", "current", ccc.AppVersion, "latest", release.Version, "url", release.URL)
	}
}
