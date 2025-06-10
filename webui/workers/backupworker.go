package workers

import (
	"context"
	"time"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/backup"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
)

// DefaultBackupWorker handles automatic backup creation in the background
type DefaultBackupWorker struct {
	backupService backup.BackupService
	config        ccc.AppConfig
	logger        ccc.Logger
	ctx           context.Context
	cancel        context.CancelFunc
}

// NewDefaultBackupWorker creates a new backup worker instance
func NewDefaultBackupWorker(backupService backup.BackupService, config ccc.AppConfig, logger ccc.Logger) *DefaultBackupWorker {
	if logger == nil {
		logger = ccc.NopLogger
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &DefaultBackupWorker{
		backupService: backupService,
		config:        config,
		logger:        logger,
		ctx:           ctx,
		cancel:        cancel,
	}
}

// Start begins the background worker loop
func (w *DefaultBackupWorker) Start() {
	w.logger.Info("Starting backup worker")

	// Check if backups are enabled
	if !w.config.Backup.Enabled {
		w.logger.Info("Backup worker disabled via configuration")
		return
	}

	if w.config.Backup.IntervalDays <= 0 {
		w.logger.Info("Backup worker disabled: invalid interval", "interval_days", w.config.Backup.IntervalDays)
		return
	}

	w.logger.Info("Backup worker started",
		"enabled", w.config.Backup.Enabled,
		"interval_days", w.config.Backup.IntervalDays,
		"max_generations", w.config.Backup.MaxGenerations)

	// Start the worker in a goroutine
	go w.run()
}

// Stop gracefully stops the backup worker
func (w *DefaultBackupWorker) Stop() {
	w.logger.Info("Stopping backup worker")
	w.cancel()
}

// run is the main worker loop that runs in the background
func (w *DefaultBackupWorker) run() {
	// Check interval - we'll check for backups every hour
	checkInterval := time.Hour

	// Create a ticker for periodic checks
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	w.logger.Info("Backup worker loop started", "check_interval", checkInterval)

	// Run initial backup check immediately
	w.performBackupCheck()

	// Main worker loop
	for {
		select {
		case <-w.ctx.Done():
			w.logger.Info("Backup worker stopped")
			return
		case <-ticker.C:
			w.performBackupCheck()
		}
	}
}

// performBackupCheck checks if a backup is needed and creates one if necessary
func (w *DefaultBackupWorker) performBackupCheck() {
	w.logger.Debug("Performing backup check")

	// Check if backup is needed
	needed, err := w.backupService.NeedsBackup()
	if err != nil {
		w.logger.Error("Failed to check if backup is needed", "error", err)
		return
	}

	if !needed {
		w.logger.Debug("No backup needed at this time")
		return
	}

	w.logger.Info("Backup needed, creating automatic backup")

	// Create the backup
	backupInfo, err := w.backupService.CreateBackup(backup.BackupTriggerAuto)
	if err != nil {
		w.logger.Error("Failed to create automatic backup", "error", err)
		return
	}

	w.logger.Info("Automatic backup created successfully",
		"filename", backupInfo.Filename,
		"size_bytes", backupInfo.SizeBytes)

	// Cleanup old backups
	w.performCleanup()
}

// performCleanup removes old backup files according to configuration
func (w *DefaultBackupWorker) performCleanup() {
	w.logger.Debug("Performing backup cleanup")

	if err := w.backupService.CleanupOldBackups(); err != nil {
		w.logger.Error("Failed to cleanup old backups", "error", err)
		return
	}

	w.logger.Debug("Backup cleanup completed successfully")
}
