package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	VSCodeDir           = ".vscode"
	VSCodeSettingsPath  = ".vscode/settings.json"
	EditorWorktreeGlob  = "**/.forest/worktrees/**"
	FilesWatcherExclude = "files.watcherExclude"
	SearchExclude       = "search.exclude"
)

func EnsureVSCodeIgnores(root string) (bool, error) {
	settings, existed, err := readVSCodeSettings(root)
	if err != nil {
		return false, err
	}
	changed := !existed
	sectionChanged, err := ensureNestedBool(settings, FilesWatcherExclude, EditorWorktreeGlob)
	if err != nil {
		return false, err
	}
	if sectionChanged {
		changed = true
	}
	sectionChanged, err = ensureNestedBool(settings, SearchExclude, EditorWorktreeGlob)
	if err != nil {
		return false, err
	}
	if sectionChanged {
		changed = true
	}
	if !changed {
		return false, nil
	}
	if err := ensureSafeVSCodePath(root); err != nil {
		return false, err
	}
	if err := os.MkdirAll(filepath.Join(root, VSCodeDir), 0o755); err != nil {
		return false, err
	}
	if err := ensureSafeVSCodePath(root); err != nil {
		return false, err
	}
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return false, err
	}
	data = append(data, '\n')
	if err := os.WriteFile(filepath.Join(root, VSCodeSettingsPath), data, 0o644); err != nil {
		return false, err
	}
	return true, nil
}

func HasVSCodeIgnores(root string) (bool, error) {
	settings, existed, err := readVSCodeSettings(root)
	if err != nil || !existed {
		return false, err
	}
	watcher, err := nestedBool(settings, FilesWatcherExclude, EditorWorktreeGlob)
	if err != nil || !watcher {
		return false, err
	}
	search, err := nestedBool(settings, SearchExclude, EditorWorktreeGlob)
	if err != nil {
		return false, err
	}
	return search, nil
}

func readVSCodeSettings(root string) (map[string]any, bool, error) {
	if err := ensureSafeVSCodePath(root); err != nil {
		return nil, false, err
	}
	path := filepath.Join(root, VSCodeSettingsPath)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string]any{}, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return map[string]any{}, true, nil
	}
	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, true, fmt.Errorf("%s is not valid JSON; JSONC comments are not rewritten automatically: %w", VSCodeSettingsPath, err)
	}
	if settings == nil {
		settings = map[string]any{}
	}
	return settings, true, nil
}

func ensureNestedBool(settings map[string]any, section, key string) (bool, error) {
	raw, exists := settings[section]
	nested, ok := raw.(map[string]any)
	if !exists {
		nested = map[string]any{}
		settings[section] = nested
	} else if !ok {
		return false, fmt.Errorf("%s in %s must be an object", section, VSCodeSettingsPath)
	}
	if rawValue, exists := nested[key]; exists {
		value, ok := rawValue.(bool)
		if !ok {
			return false, fmt.Errorf("%s.%s in %s must be a boolean", section, key, VSCodeSettingsPath)
		}
		if value {
			return false, nil
		}
		return false, fmt.Errorf("%s.%s in %s is false; remove it or set it to true", section, key, VSCodeSettingsPath)
	}
	nested[key] = true
	return true, nil
}

func nestedBool(settings map[string]any, section, key string) (bool, error) {
	raw, exists := settings[section]
	if !exists {
		return false, nil
	}
	nested, ok := raw.(map[string]any)
	if !ok {
		return false, fmt.Errorf("%s in %s must be an object", section, VSCodeSettingsPath)
	}
	value, ok := nested[key].(bool)
	return ok && value, nil
}

func ensureSafeVSCodePath(root string) error {
	dir := filepath.Join(root, VSCodeDir)
	info, err := os.Lstat(dir)
	if err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%s must not be a symlink", VSCodeDir)
		}
		if !info.IsDir() {
			return fmt.Errorf("%s must be a directory", VSCodeDir)
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	path := filepath.Join(root, VSCodeSettingsPath)
	info, err = os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%s must not be a symlink", VSCodeSettingsPath)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s must be a regular file", VSCodeSettingsPath)
	}
	return nil
}
