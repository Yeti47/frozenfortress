package backup

import (
	"time"
)

// BackupTrigger represents the type of trigger that initiated a backup
type BackupTrigger string

const (
	// BackupTriggerManual indicates the backup was created manually via CLI
	BackupTriggerManual BackupTrigger = "manual"
	// BackupTriggerAuto indicates the backup was created automatically by the background worker
	BackupTriggerAuto BackupTrigger = "auto"
)

// BackupInfo contains metadata about a backup file
type BackupInfo struct {
	Filename  string        // The backup filename (e.g., "ff_backup_20250610_143052_auto.db")
	FilePath  string        // Full path to the backup file
	CreatedAt time.Time     // When the backup was created
	Trigger   BackupTrigger // What triggered the backup (manual/auto)
	SizeBytes int64         // Size of the backup file in bytes
}

// String returns the string representation of BackupTrigger
func (bt BackupTrigger) String() string {
	return string(bt)
}

// IsValid checks if the BackupTrigger has a valid value
func (bt BackupTrigger) IsValid() bool {
	return bt == BackupTriggerManual || bt == BackupTriggerAuto
}
