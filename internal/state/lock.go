package state

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/oluwatimilehin/git-forest/internal/config"
)

type Lock struct {
	PID       int       `json:"pid"`
	Hostname  string    `json:"hostname"`
	Command   string    `json:"command"`
	CreatedAt time.Time `json:"createdAt"`
}

type LockStatus struct {
	Exists bool
	Stale  bool
	Reason string
	Lock   Lock
}

func WithLock(root, command string, fn func() error) error {
	unlock, err := acquire(root, command)
	if err != nil {
		return err
	}
	defer unlock()
	return fn()
}

func InspectLock(root string) (LockStatus, error) {
	data, err := os.ReadFile(lockPath(root))
	if os.IsNotExist(err) {
		return LockStatus{Reason: "missing"}, nil
	}
	if err != nil {
		return LockStatus{}, err
	}
	var lock Lock
	if err := json.Unmarshal(data, &lock); err != nil {
		return LockStatus{Exists: true, Stale: true, Reason: "malformed lock file"}, nil
	}
	if lock.PID <= 0 {
		return LockStatus{Exists: true, Stale: true, Reason: "missing process id", Lock: lock}, nil
	}
	host, _ := os.Hostname()
	if lock.Hostname != "" && host != "" && !strings.EqualFold(lock.Hostname, host) {
		return LockStatus{Exists: true, Reason: "held by another host", Lock: lock}, nil
	}
	if !processRunning(lock.PID) {
		return LockStatus{Exists: true, Stale: true, Reason: fmt.Sprintf("process %d is not running", lock.PID), Lock: lock}, nil
	}
	return LockStatus{Exists: true, Reason: fmt.Sprintf("held by process %d", lock.PID), Lock: lock}, nil
}

func acquire(root, command string) (func(), error) {
	lockPath := lockPath(root)
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return nil, fmt.Errorf("Forest state is locked; run forest doctor --fix if the lock is stale")
		}
		return nil, err
	}
	host, _ := os.Hostname()
	payload, _ := json.MarshalIndent(Lock{PID: os.Getpid(), Hostname: host, Command: command, CreatedAt: time.Now().UTC()}, "", "  ")
	_, writeErr := file.Write(payload)
	closeErr := file.Close()
	if writeErr != nil {
		_ = os.Remove(lockPath)
		return nil, writeErr
	}
	if closeErr != nil {
		_ = os.Remove(lockPath)
		return nil, closeErr
	}
	return func() { _ = os.Remove(lockPath) }, nil
}

func ClearLock(root string) error {
	err := os.Remove(lockPath(root))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func lockPath(root string) string {
	return filepath.Join(root, config.StateDir, "lock")
}

func processRunning(pid int) bool {
	if pid <= 0 {
		return false
	}
	if runtime.GOOS == "windows" {
		out, err := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/FO", "CSV", "/NH").Output()
		if err != nil {
			return true
		}
		return strings.Contains(string(out), strconv.Itoa(pid))
	}
	if err := exec.Command("kill", "-0", strconv.Itoa(pid)).Run(); err != nil {
		return false
	}
	return true
}
