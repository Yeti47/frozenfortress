package backup

import (
	"time"
)

// BackupService defines the interface for backup operations
type BackupService interface {
	// CreateBackup creates a new backup with the specified trigger
	CreateBackup(trigger BackupTrigger) (*BackupInfo, error)

	// ListBackups returns all available backups, sorted by creation time (newest first)
	ListBackups() ([]*BackupInfo, error)

	// DeleteBackup removes a backup file by filename
	DeleteBackup(filename string) error

	// CleanupOldBackups removes old backup files according to MaxGenerations config
	CleanupOldBackups() error

	// GetLastBackupTime returns the creation time of the most recent backup
	// Returns zero time if no backups exist
	GetLastBackupTime() (time.Time, error)

	// NeedsBackup checks if a backup should be created based on config and last backup time
	NeedsBackup() (bool, error)
}
