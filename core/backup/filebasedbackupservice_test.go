package backup

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
)

func TestVacuumIntoBackup(t *testing.T) {
	tmpDir := t.TempDir()

	dbPath := filepath.Join(tmpDir, "test.db")
	backupDir := filepath.Join(tmpDir, "backups")

	if err := os.MkdirAll(backupDir, 0755); err != nil {
		t.Fatalf("failed to create backup dir: %v", err)
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec("CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)"); err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	if _, err := db.Exec("INSERT INTO users (id, name) VALUES (1, 'Alice'), (2, 'Bob')"); err != nil {
		t.Fatalf("failed to insert data: %v", err)
	}

	config := ccc.AppConfig{
		DatabasePath: dbPath,
		Backup: ccc.BackupConfig{
			Enabled:        true,
			Directory:      backupDir,
			IntervalDays:   1,
			MaxGenerations: 5,
		},
	}

	svc := NewFileBasedBackupService(config, ccc.NopLogger)

	backupInfo, err := svc.CreateBackup(BackupTriggerManual)
	if err != nil {
		t.Fatalf("CreateBackup failed: %v", err)
	}

	if _, err := os.Stat(backupInfo.FilePath); os.IsNotExist(err) {
		t.Fatalf("backup file does not exist: %s", backupInfo.FilePath)
	}

	if backupInfo.SizeBytes == 0 {
		t.Fatal("backup file size is 0")
	}

	if backupInfo.Trigger != BackupTriggerManual {
		t.Fatalf("expected trigger manual, got %s", backupInfo.Trigger)
	}

	backupDB, err := sql.Open("sqlite3", backupInfo.FilePath)
	if err != nil {
		t.Fatalf("failed to open backup database: %v", err)
	}
	defer backupDB.Close()

	var count int
	if err := backupDB.QueryRow("SELECT COUNT(*) FROM users").Scan(&count); err != nil {
		t.Fatalf("failed to query backup: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 rows in backup, got %d", count)
	}
}
