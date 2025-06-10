package backup

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
)

// FileBasedBackupService implements BackupService using file system operations
type FileBasedBackupService struct {
	config ccc.AppConfig
	logger ccc.Logger
}

// NewFileBasedBackupService creates a new file-based backup service
func NewFileBasedBackupService(config ccc.AppConfig, logger ccc.Logger) *FileBasedBackupService {
	if logger == nil {
		logger = ccc.NopLogger
	}

	return &FileBasedBackupService{
		config: config,
		logger: logger,
	}
}

// CreateBackup creates a new backup with the specified trigger
func (s *FileBasedBackupService) CreateBackup(trigger BackupTrigger) (*BackupInfo, error) {
	s.logger.Info("Creating backup", "trigger", trigger.String())

	// Check if backups are enabled
	if !s.config.Backup.Enabled {
		s.logger.Warn("Backup creation attempted but backups are disabled")
		return nil, fmt.Errorf("backups are disabled in configuration")
	}

	// Validate trigger
	if !trigger.IsValid() {
		s.logger.Warn("Invalid backup trigger provided", "trigger", trigger.String())
		return nil, fmt.Errorf("invalid trigger: %s", trigger.String())
	}

	// Ensure backup directory exists
	if err := s.ensureBackupDirectory(); err != nil {
		s.logger.Error("Failed to ensure backup directory exists", "error", err)
		return nil, fmt.Errorf("failed to ensure backup directory: %w", err)
	}

	// Generate backup filename
	filename := s.generateBackupFilename(trigger)
	backupPath := filepath.Join(s.config.Backup.Directory, filename)

	// Create the backup
	if err := s.copyDatabase(s.config.DatabasePath, backupPath); err != nil {
		s.logger.Error("Failed to copy database for backup", "source", s.config.DatabasePath, "destination", backupPath, "error", err)
		return nil, fmt.Errorf("failed to copy database: %w", err)
	}

	// Get file info
	fileInfo, err := os.Stat(backupPath)
	if err != nil {
		s.logger.Error("Failed to get backup file info", "path", backupPath, "error", err)
		return nil, fmt.Errorf("failed to get backup file info: %w", err)
	}

	backupInfo := &BackupInfo{
		Filename:  filename,
		FilePath:  backupPath,
		CreatedAt: fileInfo.ModTime(),
		Trigger:   trigger,
		SizeBytes: fileInfo.Size(),
	}

	s.logger.Info("Backup created successfully",
		"filename", filename,
		"size_bytes", backupInfo.SizeBytes,
		"trigger", trigger.String())

	return backupInfo, nil
}

// ListBackups returns all available backups, sorted by creation time (newest first)
func (s *FileBasedBackupService) ListBackups() ([]*BackupInfo, error) {
	s.logger.Debug("Listing backups", "directory", s.config.Backup.Directory)

	// Check if backup directory exists
	if _, err := os.Stat(s.config.Backup.Directory); os.IsNotExist(err) {
		s.logger.Debug("Backup directory does not exist", "directory", s.config.Backup.Directory)
		return []*BackupInfo{}, nil
	}

	// Read directory contents
	entries, err := os.ReadDir(s.config.Backup.Directory)
	if err != nil {
		s.logger.Error("Failed to read backup directory", "directory", s.config.Backup.Directory, "error", err)
		return nil, fmt.Errorf("failed to read backup directory: %w", err)
	}

	var backups []*BackupInfo
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		backupInfo, err := s.parseBackupFile(entry.Name())
		if err != nil {
			s.logger.Warn("Failed to parse backup file", "filename", entry.Name(), "error", err)
			continue // Skip invalid backup files
		}

		// Get full file info
		fullPath := filepath.Join(s.config.Backup.Directory, entry.Name())
		fileInfo, err := entry.Info()
		if err != nil {
			s.logger.Warn("Failed to get file info for backup", "filename", entry.Name(), "error", err)
			continue
		}

		backupInfo.FilePath = fullPath
		backupInfo.CreatedAt = fileInfo.ModTime()
		backupInfo.SizeBytes = fileInfo.Size()

		backups = append(backups, backupInfo)
	}

	// Sort by creation time (newest first)
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].CreatedAt.After(backups[j].CreatedAt)
	})

	s.logger.Debug("Listed backups", "count", len(backups))
	return backups, nil
}

// DeleteBackup removes a backup file by filename
func (s *FileBasedBackupService) DeleteBackup(filename string) error {
	s.logger.Info("Deleting backup", "filename", filename)

	// Validate filename
	if filename == "" {
		return fmt.Errorf("filename cannot be empty")
	}

	// Ensure it's a valid backup filename to prevent path traversal
	if _, err := s.parseBackupFile(filename); err != nil {
		s.logger.Warn("Invalid backup filename for deletion", "filename", filename)
		return fmt.Errorf("invalid backup filename: %s", filename)
	}

	// Construct full path
	fullPath := filepath.Join(s.config.Backup.Directory, filename)

	// Check if file exists
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		s.logger.Warn("Backup file not found for deletion", "filename", filename)
		return fmt.Errorf("backup file not found: %s", filename)
	}

	// Delete the file
	if err := os.Remove(fullPath); err != nil {
		s.logger.Error("Failed to delete backup file", "filename", filename, "error", err)
		return fmt.Errorf("failed to delete backup file: %w", err)
	}

	s.logger.Info("Backup deleted successfully", "filename", filename)
	return nil
}

// CleanupOldBackups removes old backup files according to MaxGenerations config
func (s *FileBasedBackupService) CleanupOldBackups() error {
	s.logger.Debug("Starting backup cleanup", "max_generations", s.config.Backup.MaxGenerations)

	if s.config.Backup.MaxGenerations <= 0 {
		s.logger.Debug("Backup cleanup skipped: MaxGenerations is 0 or negative")
		return nil
	}

	backups, err := s.ListBackups()
	if err != nil {
		return err
	}

	if len(backups) <= s.config.Backup.MaxGenerations {
		s.logger.Debug("No cleanup needed", "current_count", len(backups), "max_generations", s.config.Backup.MaxGenerations)
		return nil
	}

	// Calculate how many to delete
	toDelete := len(backups) - s.config.Backup.MaxGenerations
	backupsToDelete := backups[s.config.Backup.MaxGenerations:] // Skip the newest ones

	s.logger.Info("Cleaning up old backups", "to_delete", toDelete, "keeping", s.config.Backup.MaxGenerations)

	for _, backup := range backupsToDelete {
		if err := s.DeleteBackup(backup.Filename); err != nil {
			s.logger.Error("Failed to delete backup during cleanup", "filename", backup.Filename, "error", err)
			// Continue with other deletions even if one fails
		}
	}

	s.logger.Info("Backup cleanup completed", "deleted", toDelete)
	return nil
}

// GetLastBackupTime returns the creation time of the most recent backup
func (s *FileBasedBackupService) GetLastBackupTime() (time.Time, error) {
	backups, err := s.ListBackups()
	if err != nil {
		return time.Time{}, err
	}

	if len(backups) == 0 {
		return time.Time{}, nil
	}

	return backups[0].CreatedAt, nil
}

// NeedsBackup checks if a backup should be created based on config and last backup time
func (s *FileBasedBackupService) NeedsBackup() (bool, error) {
	// Check if backups are enabled
	if !s.config.Backup.Enabled {
		return false, nil
	}

	// Check if interval is configured
	if s.config.Backup.IntervalDays <= 0 {
		return false, nil
	}

	lastBackupTime, err := s.GetLastBackupTime()
	if err != nil {
		return false, err
	}

	// If no backups exist, we need one
	if lastBackupTime.IsZero() {
		s.logger.Debug("No previous backups found, backup needed")
		return true, nil
	}

	// Check if enough time has passed
	timeSinceLastBackup := time.Since(lastBackupTime)
	requiredInterval := time.Duration(s.config.Backup.IntervalDays) * 24 * time.Hour

	needed := timeSinceLastBackup >= requiredInterval
	s.logger.Debug("Checked backup necessity",
		"last_backup", lastBackupTime,
		"time_since", timeSinceLastBackup,
		"required_interval", requiredInterval,
		"needed", needed)

	return needed, nil
}

// ensureBackupDirectory creates the backup directory if it doesn't exist
func (s *FileBasedBackupService) ensureBackupDirectory() error {
	if err := os.MkdirAll(s.config.Backup.Directory, 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}
	return nil
}

// generateBackupFilename creates a backup filename with timestamp and trigger
// Format: ff_backup_YYYYMMDD_HHMMSS_{trigger}.db
func (s *FileBasedBackupService) generateBackupFilename(trigger BackupTrigger) string {
	now := time.Now()
	timestamp := now.Format("20060102_150405") // YYYYMMDD_HHMMSS
	return fmt.Sprintf("ff_backup_%s_%s.db", timestamp, trigger.String())
}

// parseBackupFile extracts information from a backup filename
func (s *FileBasedBackupService) parseBackupFile(filename string) (*BackupInfo, error) {
	// Pattern: ff_backup_YYYYMMDD_HHMMSS_{trigger}.db
	pattern := `^ff_backup_(\d{8})_(\d{6})_(manual|auto)\.db$`
	re := regexp.MustCompile(pattern)

	matches := re.FindStringSubmatch(filename)
	if len(matches) != 4 {
		return nil, fmt.Errorf("filename does not match backup pattern: %s", filename)
	}

	dateStr := matches[1]
	timeStr := matches[2]
	triggerStr := matches[3]

	// Parse date and time
	timestampStr := dateStr + timeStr
	createdAt, err := time.Parse("20060102150405", timestampStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse timestamp from filename: %w", err)
	}

	trigger := BackupTrigger(triggerStr)
	if !trigger.IsValid() {
		return nil, fmt.Errorf("invalid trigger in filename: %s", triggerStr)
	}

	return &BackupInfo{
		Filename:  filename,
		CreatedAt: createdAt,
		Trigger:   trigger,
	}, nil
}

// copyDatabase copies the SQLite database file to the backup location
func (s *FileBasedBackupService) copyDatabase(sourcePath, destPath string) error {
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to open source database: %w", err)
	}
	defer sourceFile.Close()

	destFile, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create backup file: %w", err)
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return fmt.Errorf("failed to copy database: %w", err)
	}

	return nil
}
