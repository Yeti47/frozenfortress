package workers

// BackupWorker defines the interface for background backup operations
type BackupWorker interface {
	// Start begins the background worker loop
	Start()

	// Stop gracefully stops the backup worker
	Stop()
}
