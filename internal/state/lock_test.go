package state

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Timmyy3000/git-forest/internal/config"
)

func TestInspectLockReportsMissing(t *testing.T) {
	status, err := InspectLock(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if status.Exists {
		t.Fatal("expected missing lock")
	}
}

func TestInspectLockReportsStaleSameHostProcess(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, config.StateDir), 0o755); err != nil {
		t.Fatal(err)
	}
	host, _ := os.Hostname()
	data, err := json.Marshal(Lock{PID: 99999999, Hostname: host, Command: "forest test", CreatedAt: time.Now().UTC()})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lockPath(root), data, 0o644); err != nil {
		t.Fatal(err)
	}
	status, err := InspectLock(root)
	if err != nil {
		t.Fatal(err)
	}
	if !status.Exists || !status.Stale {
		t.Fatalf("expected stale lock, got %+v", status)
	}
}

func TestWithLockClearsStaleSameHostLock(t *testing.T) {
	root := t.TempDir()
	writeLock(t, root, Lock{PID: 99999999, Hostname: localHost(t), Command: "forest old", CreatedAt: time.Now().UTC()})

	called := false
	err := WithLock(root, "forest test", func() error {
		called = true
		status, err := InspectLock(root)
		if err != nil {
			t.Fatal(err)
		}
		if !status.Exists || status.Stale {
			t.Fatalf("expected fresh lock while callback runs, got %+v", status)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("expected callback to run")
	}
	status, err := InspectLock(root)
	if err != nil {
		t.Fatal(err)
	}
	if status.Exists {
		t.Fatalf("expected lock to be removed after callback, got %+v", status)
	}
}

func TestWithLockRefusesActiveLock(t *testing.T) {
	root := t.TempDir()
	writeLock(t, root, Lock{PID: os.Getpid(), Hostname: localHost(t), Command: "forest active", CreatedAt: time.Now().UTC()})

	called := false
	err := WithLock(root, "forest test", func() error {
		called = true
		return nil
	})
	if err == nil {
		t.Fatal("expected active lock error")
	}
	var lockErr *LockError
	if !errors.As(err, &lockErr) {
		t.Fatalf("expected LockError, got %T", err)
	}
	if lockErr.Status.Stale {
		t.Fatalf("expected active lock status, got %+v", lockErr.Status)
	}
	if called {
		t.Fatal("callback should not run when lock is active")
	}
}

func writeLock(t *testing.T, root string, lock Lock) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, config.StateDir), 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(lock)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lockPath(root), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func localHost(t *testing.T) string {
	t.Helper()
	host, err := os.Hostname()
	if err != nil {
		t.Fatal(err)
	}
	return host
}
