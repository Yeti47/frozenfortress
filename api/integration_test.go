package main

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
	"github.com/gomodule/redigo/redis"
)

// requireRedisAndIsolatedEnv skips when no local Redis is reachable (CI provides one) and
// points every path the app writes to (database, keys, backups, logs) at a temp dir.
func requireRedisAndIsolatedEnv(t *testing.T) string {
	t.Helper()
	conn, err := redis.Dial("tcp", "localhost:6379")
	if err != nil {
		t.Skipf("skipping: no local Redis reachable: %v", err)
	}
	conn.Close()

	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv(ccc.EnvDatabasePath, filepath.Join(dir, "ff.db"))
	t.Setenv(ccc.EnvKeyDir, filepath.Join(dir, "keys"))
	t.Setenv(ccc.EnvBackupDirectory, filepath.Join(dir, "backups"))
	t.Setenv(ccc.EnvRedisAddress, "localhost:6379")
	t.Setenv(ccc.EnvUpdateCheckEnabled, "false")
	t.Setenv(ccc.EnvOCRProvider, "nop")
	return dir
}

func freePort(t *testing.T) int {
	t.Helper()
	ln := listen(t)
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

func TestConfigureServices_WiresEverything(t *testing.T) {
	requireRedisAndIsolatedEnv(t)
	config := ccc.LoadConfigFromEnv()

	db, err := ccc.SetupDatabase(config)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	svc := configureServices(config, db)
	if svc.SignInManager == nil || svc.MekStore == nil || svc.UserManager == nil || svc.SecretManager == nil ||
		svc.TagManager == nil || svc.DocumentManager == nil || svc.ScanHandoffService == nil ||
		svc.UpdateChecker == nil || svc.BackupWorker == nil || svc.UpdateCheckWorker == nil {
		t.Fatalf("a service was not wired: %+v", svc)
	}
}

func TestRun_ServesUntilSigterm(t *testing.T) {
	requireRedisAndIsolatedEnv(t)
	port := freePort(t)
	t.Setenv(ccc.EnvAPIPort, strconv.Itoa(port))

	done := make(chan error, 1)
	go func() { done <- run() }()

	url := fmt.Sprintf("http://127.0.0.1:%d/api/system/health", port)
	waitFor(t, func() bool {
		resp, err := http.Get(url)
		if err != nil {
			return false
		}
		resp.Body.Close()
		return resp.StatusCode == http.StatusOK
	})

	// run has registered its signal handler by now: it is already serving requests.
	if err := syscall.Kill(os.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("run returned %v after SIGTERM", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("run did not shut down after SIGTERM")
	}
}

func TestRun_FailsWhenPortIsTaken(t *testing.T) {
	requireRedisAndIsolatedEnv(t)
	taken := listen(t)
	defer taken.Close()
	t.Setenv(ccc.EnvAPIPort, strconv.Itoa(taken.Addr().(*net.TCPAddr).Port))

	if err := run(); err == nil {
		t.Fatal("expected a listen error")
	}
}

func TestRun_FailsWhenDatabaseCannotBeOpened(t *testing.T) {
	dir := requireRedisAndIsolatedEnv(t)
	blocker := filepath.Join(dir, "file")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	// The database directory would have to be created below a regular file.
	t.Setenv(ccc.EnvDatabasePath, filepath.Join(blocker, "sub", "ff.db"))

	if err := run(); err == nil {
		t.Fatal("expected a database error")
	}
}
