package state

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/Timmyy3000/git-forest/internal/config"
)

func Path(root string) string {
	return filepath.Join(root, config.StateDir, "worktrees.json")
}

func Load(root string) (Store, error) {
	path := Path(root)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return NewStore(root), nil
	}
	if err != nil {
		return Store{}, err
	}
	if len(data) == 0 {
		return NewStore(root), nil
	}
	var store Store
	if err := json.Unmarshal(data, &store); err != nil {
		return Store{}, err
	}
	if store.Version == 0 {
		store.Version = 1
	}
	if store.RepoRoot == "" {
		store.RepoRoot = root
	}
	if store.Worktrees == nil {
		store.Worktrees = []Worktree{}
	}
	return store, nil
}

func Save(root string, store Store) error {
	dir := filepath.Join(root, config.StateDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(dir, "worktrees-*.tmp")
	if err != nil {
		return err
	}
	tmp := file.Name()
	defer func() { _ = os.Remove(tmp) }()
	if _, err := file.Write(append(data, '\n')); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp, 0o644); err != nil {
		return err
	}
	return atomicReplace(tmp, Path(root))
}

func AppendEvent(root string, event Event) error {
	if event.Type == "" {
		return errors.New("event type is required")
	}
	if err := os.MkdirAll(filepath.Join(root, config.StateDir), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(filepath.Join(root, config.StateDir, "events.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.Write(append(data, '\n'))
	return err
}
