package workers

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/backup"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/updates"
)

const waitTimeout = 2 * time.Second

func eventually(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(waitTimeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("condition not met in time")
}

type fakeBackupService struct {
	mu         sync.Mutex
	needs      bool
	needsErr   error
	createErr  error
	cleanupErr error
	checks     int
	creates    []backup.BackupTrigger
	cleanups   int
}

func (f *fakeBackupService) NeedsBackup() (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.checks++
	return f.needs, f.needsErr
}

func (f *fakeBackupService) CreateBackup(trigger backup.BackupTrigger) (*backup.BackupInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.creates = append(f.creates, trigger)
	if f.createErr != nil {
		return nil, f.createErr
	}
	return &backup.BackupInfo{Filename: "b.db", SizeBytes: 1}, nil
}

func (f *fakeBackupService) CleanupOldBackups() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cleanups++
	return f.cleanupErr
}

func (f *fakeBackupService) ListBackups() ([]*backup.BackupInfo, error) { return nil, nil }
func (f *fakeBackupService) DeleteBackup(string) error                  { return nil }
func (f *fakeBackupService) GetLastBackupTime() (time.Time, error)      { return time.Time{}, nil }

func (f *fakeBackupService) counts() (checks, creates, cleanups int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.checks, len(f.creates), f.cleanups
}

func backupConfig(enabled bool, days int) ccc.AppConfig {
	return ccc.AppConfig{Backup: ccc.BackupConfig{Enabled: enabled, IntervalDays: days, MaxGenerations: 3}}
}

func TestBackupWorker_NilLoggerDefaults(t *testing.T) {
	w := NewDefaultBackupWorker(&fakeBackupService{}, backupConfig(false, 7), nil)
	if w.logger == nil {
		t.Fatal("expected a default logger")
	}
}

func TestBackupWorker_DoesNotRunWhenDisabled(t *testing.T) {
	for name, cfg := range map[string]ccc.AppConfig{
		"disabled":          backupConfig(false, 7),
		"non-positive":      backupConfig(true, 0),
		"negative-interval": backupConfig(true, -1),
	} {
		t.Run(name, func(t *testing.T) {
			svc := &fakeBackupService{needs: true}
			w := NewDefaultBackupWorker(svc, cfg, ccc.NopLogger)
			w.Start()
			time.Sleep(20 * time.Millisecond)
			w.Stop()
			if checks, _, _ := svc.counts(); checks != 0 {
				t.Fatalf("worker ran %d checks", checks)
			}
		})
	}
}

func TestBackupWorker_CreatesBackupAndCleansUp(t *testing.T) {
	svc := &fakeBackupService{needs: true}
	w := NewDefaultBackupWorker(svc, backupConfig(true, 7), ccc.NopLogger)
	w.Start()
	defer w.Stop()

	eventually(t, func() bool { _, creates, cleanups := svc.counts(); return creates == 1 && cleanups == 1 })
	if svc.creates[0] != backup.BackupTriggerAuto {
		t.Fatalf("trigger = %s", svc.creates[0])
	}
}

func TestBackupWorker_NoBackupNeeded(t *testing.T) {
	svc := &fakeBackupService{}
	w := NewDefaultBackupWorker(svc, backupConfig(true, 7), ccc.NopLogger)
	w.Start()
	defer w.Stop()

	eventually(t, func() bool { checks, _, _ := svc.counts(); return checks == 1 })
	if _, creates, _ := svc.counts(); creates != 0 {
		t.Fatalf("created %d backups", creates)
	}
}

func TestBackupWorker_ErrorPaths(t *testing.T) {
	tests := []struct {
		name         string
		svc          *fakeBackupService
		wantCreates  int
		wantCleanups int
	}{
		{"check fails", &fakeBackupService{needsErr: errors.New("x"), needs: true}, 0, 0},
		{"create fails", &fakeBackupService{needs: true, createErr: errors.New("x")}, 1, 0},
		{"cleanup fails", &fakeBackupService{needs: true, cleanupErr: errors.New("x")}, 1, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := NewDefaultBackupWorker(tt.svc, backupConfig(true, 7), ccc.NopLogger)
			w.performBackupCheck()
			if _, creates, cleanups := tt.svc.counts(); creates != tt.wantCreates || cleanups != tt.wantCleanups {
				t.Fatalf("creates=%d cleanups=%d", creates, cleanups)
			}
		})
	}
}

func TestBackupWorker_ChecksAgainOnTick(t *testing.T) {
	svc := &fakeBackupService{}
	w := NewDefaultBackupWorker(svc, backupConfig(true, 7), ccc.NopLogger)
	w.checkInterval = 5 * time.Millisecond
	w.Start()
	defer w.Stop()

	eventually(t, func() bool { checks, _, _ := svc.counts(); return checks >= 3 })
}

func TestBackupWorker_StopEndsTheLoop(t *testing.T) {
	svc := &fakeBackupService{}
	w := NewDefaultBackupWorker(svc, backupConfig(true, 7), ccc.NopLogger)
	w.checkInterval = 5 * time.Millisecond
	w.Start()
	eventually(t, func() bool { checks, _, _ := svc.counts(); return checks >= 1 })

	w.Stop()
	time.Sleep(20 * time.Millisecond) // let an in-flight check finish
	before, _, _ := svc.counts()
	time.Sleep(50 * time.Millisecond)
	if after, _, _ := svc.counts(); after != before {
		t.Fatalf("worker kept running after Stop: %d -> %d", before, after)
	}
}

type fakeChecker struct {
	mu      sync.Mutex
	release *updates.ReleaseInfo
	err     error
	calls   int
	ctxErr  error
}

func (f *fakeChecker) Check(ctx context.Context) (*updates.ReleaseInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.ctxErr = ctx.Err()
	return f.release, f.err
}
func (f *fakeChecker) Latest() *updates.ReleaseInfo { return f.release }
func (f *fakeChecker) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func TestUpdateCheckWorker_NilLoggerDefaults(t *testing.T) {
	w := NewDefaultUpdateCheckWorker(&fakeChecker{}, ccc.AppConfig{}, nil)
	if w.logger == nil {
		t.Fatal("expected a default logger")
	}
}

func TestUpdateCheckWorker_DisabledDoesNotRun(t *testing.T) {
	checker := &fakeChecker{}
	w := NewDefaultUpdateCheckWorker(checker, ccc.AppConfig{UpdateCheckEnabled: false}, ccc.NopLogger)
	w.Start()
	time.Sleep(20 * time.Millisecond)
	w.Stop()
	if checker.callCount() != 0 {
		t.Fatal("disabled worker checked for updates")
	}
}

func TestUpdateCheckWorker_ChecksOnStartAndOnTick(t *testing.T) {
	checker := &fakeChecker{release: &updates.ReleaseInfo{Version: "9.9.9", URL: "u"}}
	w := NewDefaultUpdateCheckWorker(checker, ccc.AppConfig{UpdateCheckEnabled: true}, ccc.NopLogger)
	w.interval = 5 * time.Millisecond
	w.Start()
	defer w.Stop()

	eventually(t, func() bool { return checker.callCount() >= 3 })
}

func TestUpdateCheckWorker_CheckErrorIsTolerated(t *testing.T) {
	checker := &fakeChecker{err: errors.New("offline")}
	w := NewDefaultUpdateCheckWorker(checker, ccc.AppConfig{UpdateCheckEnabled: true}, ccc.NopLogger)
	w.performUpdateCheck() // must not panic
	if checker.callCount() != 1 {
		t.Fatalf("calls = %d", checker.callCount())
	}
}

func TestUpdateCheckWorker_StopCancelsContextAndLoop(t *testing.T) {
	checker := &fakeChecker{}
	w := NewDefaultUpdateCheckWorker(checker, ccc.AppConfig{UpdateCheckEnabled: true}, ccc.NopLogger)
	w.interval = 5 * time.Millisecond
	w.Start()
	eventually(t, func() bool { return checker.callCount() >= 1 })

	w.Stop()
	time.Sleep(20 * time.Millisecond)
	before := checker.callCount()
	time.Sleep(50 * time.Millisecond)
	if after := checker.callCount(); after != before {
		t.Fatalf("worker kept running after Stop: %d -> %d", before, after)
	}
	if w.ctx.Err() == nil {
		t.Fatal("Stop must cancel the worker context")
	}
}
