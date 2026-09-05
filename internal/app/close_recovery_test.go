package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Timmyy3000/git-forest/internal/config"
	"github.com/Timmyy3000/git-forest/internal/git"
	"github.com/Timmyy3000/git-forest/internal/state"
)

func TestCloseRetryMissingWorktreePreservesBranch(t *testing.T) {
	root := initGitRepo(t)
	runGit(t, root, "branch", "-M", "main")
	t.Chdir(root)
	application := New()
	added, err := application.Add(context.Background(), AddOptions{Name: "retry"})
	if err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "worktree", "remove", added.Path)
	result, err := application.Close(context.Background(), CloseOptions{Name: added.Name})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Closed) != 0 || len(result.Skipped) != 1 || !strings.Contains(result.Skipped[0].Reason, "--yes") {
		t.Fatalf("retry without confirmation = %+v", result)
	}
	result, err = application.Close(context.Background(), CloseOptions{Name: added.Name, Yes: true, DeleteBranch: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Closed) != 1 || len(result.Skipped) != 0 || len(result.Warnings) == 0 {
		t.Fatalf("retry = %+v", result)
	}
	if !git.BranchExists(context.Background(), root, added.Branch) {
		t.Fatal("stale recovery deleted branch")
	}
	store, err := state.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(store.Worktrees) != 0 {
		t.Fatal("stale record remains")
	}
}

func TestCloseSaveFailurePreservesBranchAndEvents(t *testing.T) {
	root := initGitRepo(t)
	runGit(t, root, "branch", "-M", "main")
	t.Chdir(root)
	added, err := New().Add(context.Background(), AddOptions{Name: "save-failure"})
	if err != nil {
		t.Fatal(err)
	}
	injected := errors.New("state save failed")
	failing := &App{saveFn: func(string, state.Store) error { return injected }}
	_, err = failing.Close(context.Background(), CloseOptions{Name: added.Name, Yes: true, DeleteBranch: true})
	if !errors.Is(err, injected) {
		t.Fatalf("close error = %v", err)
	}
	if _, err := os.Lstat(added.Path); !os.IsNotExist(err) {
		t.Fatalf("removal did not complete: %v", err)
	}
	if !git.BranchExists(context.Background(), root, added.Branch) {
		t.Fatal("branch deleted before state save")
	}
	store, err := state.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, ok := store.Find(added.Name); !ok {
		t.Fatal("failed save lost record")
	}
	events, err := os.ReadFile(filepath.Join(root, config.StateDir, "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(events), `"type":"closed"`) {
		t.Fatal("closed event emitted before state save")
	}
	result, err := New().Close(context.Background(), CloseOptions{Name: added.Name, Yes: true, DeleteBranch: true})
	if err != nil || len(result.Closed) != 1 {
		t.Fatalf("retry = %+v, %v", result, err)
	}
	if !git.BranchExists(context.Background(), root, added.Branch) {
		t.Fatal("retry deleted branch")
	}
}

func TestCloseReportsEventFailureAfterSaving(t *testing.T) {
	root := initGitRepo(t)
	runGit(t, root, "branch", "-M", "main")
	t.Chdir(root)
	added, err := New().Add(context.Background(), AddOptions{Name: "event-failure"})
	if err != nil {
		t.Fatal(err)
	}
	eventsPath := filepath.Join(root, config.StateDir, "events.jsonl")
	if err := os.Remove(eventsPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(eventsPath, 0o755); err != nil {
		t.Fatal(err)
	}
	result, err := New().Close(context.Background(), CloseOptions{Name: added.Name, Yes: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Closed) != 1 || len(result.Warnings) != 1 || !strings.Contains(result.Warnings[0], "event") {
		t.Fatalf("close = %+v", result)
	}
	store, err := state.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(store.Worktrees) != 0 {
		t.Fatal("successful close not persisted")
	}
}

func TestVerifyWorktreeRemovalUsesLiveRegistrationAndDisk(t *testing.T) {
	root := initGitRepo(t)
	path := filepath.Join(root, "worktree")
	runGit(t, root, "worktree", "add", "-b", "feature", path, "HEAD")
	err := verifyWorktreeRemoval(context.Background(), root, path)
	if err == nil || !strings.Contains(err.Error(), "registration still exists") || !strings.Contains(err.Error(), "path still exists") {
		t.Fatalf("registered directory = %v", err)
	}
	// Move only a disposable fixture directory to model missing disk with retained metadata.
	moved := filepath.Join(root, "moved")
	if err := os.Rename(path, moved); err != nil {
		t.Fatal(err)
	}
	err = verifyWorktreeRemoval(context.Background(), root, path)
	if err == nil || !strings.Contains(err.Error(), "registration still exists") || !strings.Contains(err.Error(), "path absent") {
		t.Fatalf("registration alone = %v", err)
	}
	if err := os.Rename(moved, path); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "worktree", "remove", path)
	if err := verifyWorktreeRemoval(context.Background(), root, path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}
	err = verifyWorktreeRemoval(context.Background(), root, path)
	if err == nil || !strings.Contains(err.Error(), "registration absent") || !strings.Contains(err.Error(), "path still exists") {
		t.Fatalf("residual alone = %v", err)
	}
}

func TestVerifyWorktreeRemovalDoesNotTreatErrorsAsAbsence(t *testing.T) {
	root := initGitRepo(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := verifyWorktreeRemoval(ctx, root, filepath.Join(root, "absent")); err == nil || !strings.Contains(err.Error(), "registration unknown") {
		t.Fatalf("cancelled inspection = %v", err)
	}
	if err := verifyWorktreeRemoval(context.Background(), root, filepath.Join(root, "invalid\x00path")); err == nil || !strings.Contains(err.Error(), "path unknown") {
		t.Fatalf("invalid path inspection = %v", err)
	}
}
