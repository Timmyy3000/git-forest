package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDefaultCopyListIsSmallAndSafe(t *testing.T) {
	want := []string{".env", ".env.local"}
	if got := Default().Copy; !reflect.DeepEqual(got, want) {
		t.Fatalf("Default().Copy = %#v, want %#v", got, want)
	}
}

func TestEnsureWritesSmallSafeCopyList(t *testing.T) {
	root := t.TempDir()
	if err := Ensure(root); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{".env", ".env.local"}
	if !reflect.DeepEqual(cfg.Copy, want) {
		t.Fatalf("config copy = %#v, want %#v", cfg.Copy, want)
	}
}

func TestRepairLegacyCopyDefaultUpdatesOnlyGeneratedLegacyList(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ConfigPath), "[add]\ncopy = [\".env\", \".env.local\", \".claude\", \".cursor\", \".agent\", \"skills\"]\n")

	changed, err := RepairLegacyCopyDefault(root)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("expected legacy config to be updated")
	}
	cfg, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{".env", ".env.local"}
	if !reflect.DeepEqual(cfg.Copy, want) {
		t.Fatalf("config copy = %#v, want %#v", cfg.Copy, want)
	}
}

func TestRepairLegacyCopyDefaultPreservesCustomCopyList(t *testing.T) {
	root := t.TempDir()
	custom := "[add]\ncopy = [\".env\", \"skills\"]\n"
	writeFile(t, filepath.Join(root, ConfigPath), custom)

	changed, err := RepairLegacyCopyDefault(root)
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("custom config should not be changed")
	}
	data, err := os.ReadFile(filepath.Join(root, ConfigPath))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != custom {
		t.Fatalf("custom config changed: %q", data)
	}
}

func TestCopyReusableSkipsClaudeWorktreesWhenClaudeIsOptedIn(t *testing.T) {
	root := t.TempDir()
	worktree := filepath.Join(root, ".forest", "worktrees", "feature", "x")
	writeFile(t, filepath.Join(root, ".claude", "settings.json"), "settings")
	writeFile(t, filepath.Join(root, ".claude", "worktrees", "agent", "README.md"), "nested checkout")

	copied, warnings := CopyReusable(root, worktree, Config{Copy: []string{".claude"}})
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %#v", warnings)
	}
	if !reflect.DeepEqual(copied, []string{".claude"}) {
		t.Fatalf("copied = %#v", copied)
	}
	if _, err := os.Stat(filepath.Join(worktree, ".claude", "settings.json")); err != nil {
		t.Fatalf("expected .claude/settings.json to be copied: %v", err)
	}
	if _, err := os.Stat(filepath.Join(worktree, ".claude", "worktrees")); !os.IsNotExist(err) {
		t.Fatalf("expected .claude/worktrees to be skipped, got err=%v", err)
	}
}

func writeFile(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}
