package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCloseUnknownWorktreeErrors(t *testing.T) {
	root := initGitRepo(t)
	runGit(t, root, "branch", "-M", "main")
	t.Chdir(root)
	application := New()

	if _, err := application.Add(context.Background(), AddOptions{Name: "real", Agent: "test"}); err != nil {
		t.Fatal(err)
	}

	_, err := application.Close(context.Background(), CloseOptions{Name: "ghost", Yes: true})
	if err == nil {
		t.Fatal("expected close to fail for a name not in Forest state")
	}
	if !strings.Contains(err.Error(), "unknown worktree ghost") {
		t.Fatalf("error = %q, want it to name the unknown worktree", err)
	}

	list, err := application.List(context.Background(), ListOptions{Fast: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Worktrees) != 1 || list.Worktrees[0].Name != "real" {
		t.Fatalf("existing worktrees must be untouched, got %+v", list.Worktrees)
	}
}

func TestCloseIncludeDirtyForcesRemoval(t *testing.T) {
	root := initGitRepo(t)
	runGit(t, root, "branch", "-M", "main")
	t.Chdir(root)
	application := New()

	added, err := application.Add(context.Background(), AddOptions{Name: "grubby", Agent: "test"})
	if err != nil {
		t.Fatal(err)
	}
	wtPath := added.Path
	if !filepath.IsAbs(wtPath) {
		wtPath = filepath.Join(root, wtPath)
	}
	if err := os.WriteFile(filepath.Join(wtPath, "scratch.txt"), []byte("wip\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := application.Close(context.Background(), CloseOptions{Name: "grubby", Yes: true, IncludeDirty: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Closed) != 1 || result.Closed[0] != "grubby" {
		t.Fatalf("expected grubby closed, got closed=%v skipped=%v", result.Closed, result.Skipped)
	}
	if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
		t.Fatalf("worktree directory should be removed, stat err = %v", err)
	}
	out := runGitOutput(t, root, "worktree", "list", "--porcelain")
	if strings.Contains(out, "grubby") {
		t.Fatalf("git should no longer track the worktree:\n%s", out)
	}
}

func TestCloseDirtyWithoutIncludeDirtySkips(t *testing.T) {
	root := initGitRepo(t)
	runGit(t, root, "branch", "-M", "main")
	t.Chdir(root)
	application := New()

	added, err := application.Add(context.Background(), AddOptions{Name: "guarded", Agent: "test"})
	if err != nil {
		t.Fatal(err)
	}
	wtPath := added.Path
	if !filepath.IsAbs(wtPath) {
		wtPath = filepath.Join(root, wtPath)
	}
	if err := os.WriteFile(filepath.Join(wtPath, "scratch.txt"), []byte("wip\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := application.Close(context.Background(), CloseOptions{Name: "guarded", Yes: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Closed) != 0 {
		t.Fatalf("dirty worktree must not be closed without --include-dirty, got %v", result.Closed)
	}
	if len(result.Skipped) != 1 || result.Skipped[0].Reason != "dirty" {
		t.Fatalf("expected a dirty skip, got %+v", result.Skipped)
	}
	if _, err := os.Stat(wtPath); err != nil {
		t.Fatalf("worktree directory must survive, stat err = %v", err)
	}
}

func TestCloseMergedSkipsUnmergedWorktrees(t *testing.T) {
	root := initGitRepo(t)
	runGit(t, root, "branch", "-M", "main")
	t.Chdir(root)
	application := New()

	added, err := application.Add(context.Background(), AddOptions{Name: "active", Agent: "test"})
	if err != nil {
		t.Fatal(err)
	}
	commitTestFile(t, added.Path, "active.txt", "work in progress\n", "active change")

	result, err := application.Close(context.Background(), CloseOptions{Merged: true, Yes: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Closed) != 0 {
		t.Fatalf("unmerged worktree must not be closed by --merged, got %v", result.Closed)
	}
	if len(result.Skipped) != 1 || result.Skipped[0].Reason != "not merged" {
		t.Fatalf("expected a not-merged skip, got %+v", result.Skipped)
	}
	if _, err := os.Stat(added.Path); err != nil {
		t.Fatalf("worktree directory must survive, stat err = %v", err)
	}
}
