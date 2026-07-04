package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureVSCodeIgnoresCreatesVisibleWorktreeIgnores(t *testing.T) {
	root := t.TempDir()

	changed, err := EnsureVSCodeIgnores(root)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("expected settings to change")
	}

	settings := readSettingsFile(t, root)
	assertNestedBool(t, settings, FilesWatcherExclude, EditorWorktreeGlob)
	assertNestedBool(t, settings, SearchExclude, EditorWorktreeGlob)
	if _, ok := settings["files.exclude"]; ok {
		t.Fatal("worktrees should stay visible in the file explorer")
	}

	changed, err = EnsureVSCodeIgnores(root)
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("expected configured settings to be unchanged")
	}
}

func TestEnsureVSCodeIgnoresMergesExistingSettings(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, VSCodeDir), 0o755); err != nil {
		t.Fatal(err)
	}
	existing := []byte(`{
  "editor.formatOnSave": true,
  "files.watcherExclude": {
    "**/.git/objects/**": true
  }
}`)
	if err := os.WriteFile(filepath.Join(root, VSCodeSettingsPath), existing, 0o644); err != nil {
		t.Fatal(err)
	}

	changed, err := EnsureVSCodeIgnores(root)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("expected settings to change")
	}

	settings := readSettingsFile(t, root)
	if settings["editor.formatOnSave"] != true {
		t.Fatal("expected unrelated settings to be preserved")
	}
	assertNestedBool(t, settings, FilesWatcherExclude, "**/.git/objects/**")
	assertNestedBool(t, settings, FilesWatcherExclude, EditorWorktreeGlob)
	assertNestedBool(t, settings, SearchExclude, EditorWorktreeGlob)
}

func TestHasVSCodeIgnoresReportsMissing(t *testing.T) {
	ok, err := HasVSCodeIgnores(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected missing settings to report false")
	}
}

func TestHasVSCodeIgnoresReportsConfigured(t *testing.T) {
	root := t.TempDir()
	if _, err := EnsureVSCodeIgnores(root); err != nil {
		t.Fatal(err)
	}

	ok, err := HasVSCodeIgnores(root)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected configured settings to report true")
	}
}

func TestHasVSCodeIgnoresReportsPartialConfiguration(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, VSCodeDir), 0o755); err != nil {
		t.Fatal(err)
	}
	existing := []byte(`{
  "files.watcherExclude": {
    "**/.forest/worktrees/**": true
  }
}`)
	if err := os.WriteFile(filepath.Join(root, VSCodeSettingsPath), existing, 0o644); err != nil {
		t.Fatal(err)
	}

	ok, err := HasVSCodeIgnores(root)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected partial settings to report false")
	}
}

func TestEnsureVSCodeIgnoresRejectsNonObjectSettings(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, VSCodeDir), 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, VSCodeSettingsPath)
	existing := []byte(`{"search.exclude": true}`)
	if err := os.WriteFile(path, existing, 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := EnsureVSCodeIgnores(root); err == nil {
		t.Fatal("expected invalid settings to return an error")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(existing) {
		t.Fatal("expected invalid setting to be preserved")
	}
}

func TestEnsureVSCodeIgnoresRejectsExplicitFalseWorktreeSetting(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, VSCodeDir), 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, VSCodeSettingsPath)
	existing := []byte(`{"search.exclude":{"**/.forest/worktrees/**":false}}`)
	if err := os.WriteFile(path, existing, 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := EnsureVSCodeIgnores(root); err == nil {
		t.Fatal("expected explicit false setting to return an error")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(existing) {
		t.Fatal("expected explicit false setting to be preserved")
	}
}

func TestEnsureVSCodeIgnoresRejectsCommentedSettings(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, VSCodeDir), 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, VSCodeSettingsPath)
	existing := []byte("{\n  // JSONC comments are preserved by refusing to rewrite.\n}\n")
	if err := os.WriteFile(path, existing, 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := EnsureVSCodeIgnores(root); err == nil {
		t.Fatal("expected commented JSONC settings to return an error")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(existing) {
		t.Fatal("expected commented settings to be preserved")
	}
}

func TestEnsureVSCodeIgnoresRejectsSymlinkedVSCodeDir(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, VSCodeDir)); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	if _, err := EnsureVSCodeIgnores(root); err == nil {
		t.Fatal("expected symlinked .vscode directory to return an error")
	}
}

func TestEnsureVSCodeIgnoresRejectsSymlinkedSettingsFile(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(outside, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, VSCodeDir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, VSCodeSettingsPath)); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	if _, err := EnsureVSCodeIgnores(root); err == nil {
		t.Fatal("expected symlinked settings file to return an error")
	}
}

func readSettingsFile(t *testing.T, root string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, VSCodeSettingsPath))
	if err != nil {
		t.Fatal(err)
	}
	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatal(err)
	}
	return settings
}

func assertNestedBool(t *testing.T, settings map[string]any, section, key string) {
	t.Helper()
	nested, ok := settings[section].(map[string]any)
	if !ok {
		t.Fatalf("expected %s to be an object", section)
	}
	if value, ok := nested[key].(bool); !ok || !value {
		t.Fatalf("expected %s.%s to be true", section, key)
	}
}
