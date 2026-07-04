package state

import (
	"encoding/json"
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
