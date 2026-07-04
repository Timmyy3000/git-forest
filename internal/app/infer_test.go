package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Timmyy3000/git-forest/internal/config"
	"github.com/Timmyy3000/git-forest/internal/state"
)

func TestInferCurrentSelectsDeepestContainingWorktree(t *testing.T) {
	root := t.TempDir()
	parent := filepath.Join(config.WorktreeDir, "feat")
	child := filepath.Join(config.WorktreeDir, "feat", "login")
	cwd := filepath.Join(root, child, "subdir")
	for _, dir := range []string{filepath.Join(root, parent), cwd} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })
	if err := os.Chdir(cwd); err != nil {
		t.Fatal(err)
	}

	name, err := inferCurrent(state.Store{Worktrees: []state.Worktree{
		{ID: "parent", Path: parent},
		{ID: "child", Path: child},
	}}, root)
	if err != nil {
		t.Fatal(err)
	}
	if name != "child" {
		t.Fatalf("name = %q, want child", name)
	}
}
