package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/oluwatimilehin/git-forest/internal/config"
)

type Lock struct {
	PID       int       `json:"pid"`
	Hostname  string    `json:"hostname"`
	Command   string    `json:"command"`
	CreatedAt time.Time `json:"createdAt"`
}

func WithLock(root, command string, fn func() error) error {
	unlock, err := acquire(root, command)
	if err != nil {
		return err
	}
	defer unlock()
	return fn()
}

func acquire(root, command string) (func(), error) {
	lockPath := filepath.Join(root, config.StateDir, "lock")
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
	return os.Remove(filepath.Join(root, config.StateDir, "lock"))
}
