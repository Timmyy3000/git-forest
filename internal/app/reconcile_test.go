package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Timmyy3000/git-forest/internal/config"
	"github.com/Timmyy3000/git-forest/internal/state"
)

func TestListClassifiesStaleAndFastUnverifiedWorktrees(t *testing.T) {
	root := initGitRepo(t)
	if err := config.Ensure(root); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, config.WorktreeDir, "feature", "stale")
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	store := state.NewStore(root)
	store.Worktrees = append(store.Worktrees, state.Worktree{
		ID:     "feature/stale",
		Name:   "feature/stale",
		Branch: "feature/stale",
		Path:   filepath.Join(config.WorktreeDir, "feature", "stale"),
	})
	if err := state.Save(root, store); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)

	application := New()
	listed, err := application.List(context.Background(), ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(listed.Worktrees) != 1 {
		t.Fatalf("listed worktrees = %d, want 1", len(listed.Worktrees))
	}
	stale := listed.Worktrees[0]
	if stale.Lifecycle != "stale" || stale.RegistrationStatus != "missing" {
		t.Fatalf("stale view = %+v, want stale/missing", stale)
	}
	if !strings.Contains(stale.Reason, ".git marker is missing") {
		t.Fatalf("stale reason = %q", stale.Reason)
	}
	if stale.ChecksIncomplete == false || stale.Integration != "unknown" {
		t.Fatalf("stale checks = %+v, want incomplete unknown", stale)
	}

	fast, err := application.List(context.Background(), ListOptions{Fast: true})
	if err != nil {
		t.Fatal(err)
	}
	fastView := fast.Worktrees[0]
	if fastView.Lifecycle != "unverified" || fastView.RegistrationStatus != "notChecked" {
		t.Fatalf("fast view = %+v, want unverified/notChecked", fastView)
	}
	if fastView.Reason != "Git worktree registration not checked (--fast)" {
		t.Fatalf("fast reason = %q", fastView.Reason)
	}
}

func TestDoctorFixPreservesNonEmptyStaleResidual(t *testing.T) {
	root := initGitRepo(t)
	if err := config.Ensure(root); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, config.WorktreeDir, "feature", "nonempty")
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	content := filepath.Join(path, "user.txt")
	if err := os.WriteFile(content, []byte("keep me\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	store := state.NewStore(root)
	store.Worktrees = append(store.Worktrees, state.Worktree{
		ID:     "feature/nonempty",
		Name:   "feature/nonempty",
		Branch: "feature/nonempty",
		Path:   filepath.Join(config.WorktreeDir, "feature", "nonempty"),
	})
	if err := state.Save(root, store); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)

	result, err := New().Doctor(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	if !hasCheckPrefix(result, "worktree feature/nonempty residual cleanup", "skipped:") && !hasCheckPrefix(result, "worktree feature/nonempty residual cleanup", "failed:") {
		t.Fatalf("expected non-empty residual to be preserved, got %#v", result.Checks)
	}
	if _, err := os.Stat(content); err != nil {
		t.Fatalf("user content was removed: %v", err)
	}
	store, err = state.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, ok := store.Find("feature/nonempty"); !ok {
		t.Fatal("non-empty stale residual should remain in state for manual review")
	}
}

func TestCloseSkipsNonEmptyStaleResidual(t *testing.T) {
	root := initGitRepo(t)
	if err := config.Ensure(root); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, config.WorktreeDir, "feature", "nonempty")
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	content := filepath.Join(path, "user.txt")
	if err := os.WriteFile(content, []byte("keep me\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	store := state.NewStore(root)
	store.Worktrees = append(store.Worktrees, state.Worktree{
		ID:     "feature/nonempty",
		Name:   "feature/nonempty",
		Branch: "feature/nonempty",
		Path:   filepath.Join(config.WorktreeDir, "feature", "nonempty"),
	})
	if err := state.Save(root, store); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)

	result, err := New().Close(context.Background(), CloseOptions{Name: "feature/nonempty", Yes: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Closed) != 0 || len(result.Skipped) != 1 {
		t.Fatalf("close result = %+v, want one skipped stale residual", result)
	}
	if !strings.Contains(result.Skipped[0].Reason, "not empty") {
		t.Fatalf("skip reason = %q, want non-empty diagnostic", result.Skipped[0].Reason)
	}
	if _, err := os.Stat(content); err != nil {
		t.Fatalf("user content was removed: %v", err)
	}
}
