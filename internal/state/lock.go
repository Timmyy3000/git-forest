package state

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/Timmyy3000/git-forest/internal/config"
)

type Lock struct {
	PID       int       `json:"pid"`
	Hostname  string    `json:"hostname"`
	Command   string    `json:"command"`
	CreatedAt time.Time `json:"createdAt"`
}

type LockStatus struct {
	Exists bool   `json:"exists"`
	Stale  bool   `json:"stale"`
	Reason string `json:"reason"`
	Lock   Lock   `json:"lock"`
}

type LockError struct {
	Status LockStatus
}

func (e *LockError) Error() string {
	if e == nil {
		return "Forest state is locked; wait and retry"
	}
	switch {
	case !e.Status.Exists:
		return "Forest state lock was removed concurrently; retry"
	case e.Status.Stale:
		return "Forest state lock is stale; retry or run forest doctor --fix if it persists"
	default:
		return "Forest state is locked by an active process; wait and retry"
	}
}

func WithLock(root, command string, fn func() error) error {
	unlock, err := acquire(root, command)
	if err != nil {
		return err
	}
	stopSignals := cleanupLockOnSignal(unlock)
	defer unlock()
	defer stopSignals()
	return fn()
}

func cleanupLockOnSignal(unlock func()) func() {
	signals := []os.Signal{os.Interrupt}
	if runtime.GOOS != "windows" {
		signals = append(signals, syscall.SIGTERM, syscall.SIGHUP)
	}
	ch := make(chan os.Signal, 1)
	done := make(chan struct{})
	signal.Notify(ch, signals...)
	go func() {
		select {
		case <-ch:
			unlock()
			os.Exit(130)
		case <-done:
		}
	}()
	return func() {
		signal.Stop(ch)
		close(done)
	}
}

func InspectLock(root string) (LockStatus, error) {
	data, err := os.ReadFile(lockPath(root))
	if os.IsNotExist(err) {
		return LockStatus{Reason: "missing"}, nil
	}
	if err != nil {
		return LockStatus{}, err
	}
	return inspectLockData(data), nil
}

func inspectLockData(data []byte) LockStatus {
	var lock Lock
	if err := json.Unmarshal(data, &lock); err != nil {
		return LockStatus{Exists: true, Stale: true, Reason: "malformed lock file"}
	}
	if lock.PID <= 0 {
		return LockStatus{Exists: true, Stale: true, Reason: "missing process id", Lock: lock}
	}
	reasonPrefix := ""
	host, hostErr := os.Hostname()
	if hostErr != nil {
		reasonPrefix = "cannot determine local host: " + hostErr.Error() + "; "
	}
	if lock.Hostname != "" {
		if hostErr != nil || host == "" {
			return LockStatus{Exists: true, Reason: reasonPrefix + "held by host " + lock.Hostname, Lock: lock}
		}
		if !strings.EqualFold(lock.Hostname, host) {
			return LockStatus{Exists: true, Reason: "held by another host", Lock: lock}
		}
	}
	if !processRunning(lock.PID) {
		return LockStatus{Exists: true, Stale: true, Reason: fmt.Sprintf("%sprocess %d is not running", reasonPrefix, lock.PID), Lock: lock}
	}
	return LockStatus{Exists: true, Reason: fmt.Sprintf("%sheld by process %d", reasonPrefix, lock.PID), Lock: lock}
}

func acquire(root, command string) (func(), error) {
	// Every successful stale-lock clear is followed by another tryAcquire;
	// the bound only limits how many recovery cycles run under lock churn.
	const maxRecoveries = 3
	for recovery := 0; ; recovery++ {
		unlock, err := tryAcquire(root, command)
		if err == nil {
			return unlock, nil
		}
		var lockErr *LockError
		if !errors.As(err, &lockErr) || !lockErr.Status.Stale {
			return nil, err
		}
		if recovery == maxRecoveries {
			return nil, err
		}
		status, cleared, clearErr := ClearStaleLock(root)
		if clearErr != nil {
			return nil, fmt.Errorf("stale Forest state lock detected but recovery failed: %w", clearErr)
		}
		if status.Exists && !cleared {
			return nil, &LockError{Status: status}
		}
	}
}

func tryAcquire(root, command string) (func(), error) {
	lockPath := lockPath(root)
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		if os.IsExist(err) {
			status, inspectErr := InspectLock(root)
			if inspectErr != nil {
				return nil, inspectErr
			}
			return nil, &LockError{Status: status}
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

func ClearStaleLock(root string) (LockStatus, bool, error) {
	// Fast path: no lock file, nothing to clear.
	if _, err := os.Stat(lockPath(root)); os.IsNotExist(err) {
		return LockStatus{Reason: "missing"}, false, nil
	}
	// A peer process may observe a lock between file creation and payload write.
	// Re-check before clearing so fresh locks are not mistaken for stale ones.
	time.Sleep(100 * time.Millisecond)
	data, err := os.ReadFile(lockPath(root))
	if os.IsNotExist(err) {
		return LockStatus{Reason: "missing"}, false, nil
	}
	if err != nil {
		return LockStatus{}, false, err
	}
	status := inspectLockData(data)
	if !status.Stale {
		return status, false, nil
	}
	// Re-read immediately before removal and only delete when the contents
	// still match the stale payload that was inspected. A peer that replaced
	// the lock in the meantime keeps its lock and its fresh status is
	// reported instead. This narrows (but cannot fully close) the
	// inspect-then-remove race inherent to file-based advisory locks.
	current, err := os.ReadFile(lockPath(root))
	if os.IsNotExist(err) {
		return LockStatus{Reason: "missing"}, false, nil
	}
	if err != nil {
		return status, false, err
	}
	if !bytes.Equal(current, data) {
		return inspectLockData(current), false, nil
	}
	if err := ClearLock(root); err != nil {
		return status, false, err
	}
	return status, true, nil
}

func lockPath(root string) string {
	return filepath.Join(root, config.StateDir, "lock")
}
