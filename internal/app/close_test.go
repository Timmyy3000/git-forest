package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Timmyy3000/git-forest/internal/state"
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

func TestCloseWithoutYesReportsDirty(t *testing.T) {
	root := initGitRepo(t)
	runGit(t, root, "branch", "-M", "main")
	t.Chdir(root)
	application := New()

	added, err := application.Add(context.Background(), AddOptions{Name: "dirty-no-yes", Agent: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(added.Path, "scratch.txt"), []byte("wip\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := application.Close(context.Background(), CloseOptions{Name: added.Name})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Skipped) != 1 || result.Skipped[0].Reason != "dirty" {
		t.Fatalf("close skipped = %+v, want dirty diagnostic", result.Skipped)
	}
}

func TestCloseWithoutYesReportsUnmerged(t *testing.T) {
	root := initGitRepo(t)
	runGit(t, root, "branch", "-M", "main")
	t.Chdir(root)
	application := New()

	added, err := application.Add(context.Background(), AddOptions{Name: "unmerged-no-yes", Agent: "test"})
	if err != nil {
		t.Fatal(err)
	}
	commitTestFile(t, added.Path, "active.txt", "work in progress\n", "active change")

	result, err := application.Close(context.Background(), CloseOptions{Name: added.Name})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Skipped) != 1 || result.Skipped[0].Reason != "unmerged" {
		t.Fatalf("close skipped = %+v, want unmerged diagnostic", result.Skipped)
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
	store, err := state.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, ok := store.Find(added.Name); !ok {
		t.Fatalf("skipped worktree %q must remain in state", added.Name)
	}
}

func TestCloseMergedIncludeUnmergedWarns(t *testing.T) {
	root := initGitRepo(t)
	runGit(t, root, "branch", "-M", "main")
	t.Chdir(root)
	application := New()

	added, err := application.Add(context.Background(), AddOptions{Name: "active", Agent: "test"})
	if err != nil {
		t.Fatal(err)
	}
	commitTestFile(t, added.Path, "active.txt", "work in progress\n", "active change")

	result, err := application.Close(context.Background(), CloseOptions{Merged: true, IncludeUnmerged: true, Yes: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Closed) != 1 || result.Closed[0] != added.Name {
		t.Fatalf("close result = %+v, want unmerged worktree closed", result)
	}
	if len(result.Warnings) != 1 || !strings.Contains(result.Warnings[0], "closing unmerged") {
		t.Fatalf("close warnings = %v, want explicit unmerged warning", result.Warnings)
	}
}

func TestCloseMergedHonorsName(t *testing.T) {
	root := initGitRepo(t)
	runGit(t, root, "branch", "-M", "main")
	t.Chdir(root)
	application := New()

	first, err := application.Add(context.Background(), AddOptions{Name: "first", Agent: "test"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := application.Add(context.Background(), AddOptions{Name: "second", Agent: "test"})
	if err != nil {
		t.Fatal(err)
	}

	result, err := application.Close(context.Background(), CloseOptions{Merged: true, Name: first.Name, Yes: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Closed) != 1 || result.Closed[0] != first.Name {
		t.Fatalf("merged close = %+v, want only %q closed", result, first.Name)
	}
	if _, err := os.Stat(second.Path); err != nil {
		t.Fatalf("non-selected worktree must survive, stat err = %v", err)
	}
	store, err := state.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, ok := store.Find(second.Name); !ok {
		t.Fatalf("non-selected worktree %q must remain in state", second.Name)
	}
	if _, _, ok := store.Find(first.Name); ok {
		t.Fatalf("closed worktree %q remains in state", first.Name)
	}
}

func TestCloseWithMissingBranchUsesWorktreeHead(t *testing.T) {
	root := initGitRepo(t)
	runGit(t, root, "branch", "-M", "main")
	t.Chdir(root)
	application := New()

	added, err := application.Add(context.Background(), AddOptions{Name: "missing-branch", Agent: "test"})
	if err != nil {
		t.Fatal(err)
	}
	store, err := state.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	store.Worktrees[0].Branch = ""
	if err := state.Save(root, store); err != nil {
		t.Fatal(err)
	}
	head := runGitOutput(t, added.Path, "rev-parse", "HEAD")
	baseHead := runGitOutput(t, root, "rev-parse", "refs/heads/main")
	if head != baseHead {
		t.Fatalf("worktree HEAD = %q, want main branch tip %q", head, baseHead)
	}
	listed, err := application.List(context.Background(), ListOptions{Name: added.Name})
	if err != nil {
		t.Fatal(err)
	}
	if listed.Worktrees[0].Integration != "merged" || listed.Worktrees[0].IntegrationError != "" {
		t.Fatalf("list integration = %q (%q), want merged without error", listed.Worktrees[0].Integration, listed.Worktrees[0].IntegrationError)
	}

	result, err := application.Close(context.Background(), CloseOptions{Name: added.Name, Yes: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Closed) != 1 || result.Closed[0] != added.Name {
		t.Fatalf("close result = %+v, want %q closed", result, added.Name)
	}
	store, err = state.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, ok := store.Find(added.Name); ok {
		t.Fatalf("closed worktree %q remains in state", added.Name)
	}
}
