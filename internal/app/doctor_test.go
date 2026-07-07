package app

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Timmyy3000/git-forest/internal/config"
	"github.com/Timmyy3000/git-forest/internal/pathutil"
	"github.com/Timmyy3000/git-forest/internal/state"
)

func TestValidStatePathAcceptsGeneratedWorktreePaths(t *testing.T) {
	fromName, err := pathutil.FromName(config.WorktreeDir, "fix-login")
	if err != nil {
		t.Fatal(err)
	}
	fromBranch, err := pathutil.FromBranch(config.WorktreeDir, "feat/login-copy")
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name string
		path string
	}{
		{name: "from name", path: fromName.RelPath},
		{name: "from branch", path: fromBranch.RelPath},
		{name: "nested", path: filepath.Join(config.WorktreeDir, "chore", "docs")},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if !validStatePath(tt.path) {
				t.Fatalf("expected %s to be valid", tt.path)
			}
		})
	}
}

func TestValidStatePathRejectsUnsafePaths(t *testing.T) {
	for _, path := range []string{"", ".", "../escape", config.WorktreeDir, filepath.Join(".forest", "other", "fix-login"), filepath.Join(config.WorktreeDir, "..", "escape")} {
		if validStatePath(path) {
			t.Fatalf("expected %s to be invalid", path)
		}
	}
}

func TestDoctorFixAdoptsGitWorktreeMissingFromState(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init")
	runGit(t, root, "config", "user.email", "test@example.com")
	runGit(t, root, "config", "user.name", "Forest Test")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", "README.md")
	runGit(t, root, "commit", "-m", "init")
	if err := config.Ensure(root); err != nil {
		t.Fatal(err)
	}
	if err := state.Save(root, state.NewStore(root)); err != nil {
		t.Fatal(err)
	}

	worktreePath := filepath.Join(root, config.WorktreeDir, "feature", "orphan")
	runGit(t, root, "worktree", "add", "-b", "feature/orphan", worktreePath, "HEAD")
	t.Chdir(root)

	result, err := New().Doctor(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if !hasCheck(result, "worktree feature/orphan", "untracked by Forest state") {
		t.Fatalf("expected untracked worktree check, got %#v", result.Checks)
	}

	result, err = New().Doctor(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	if !hasCheckPrefix(result, "worktree adoption", "adopted 1") {
		t.Fatalf("expected adoption check, got %#v", result.Checks)
	}
	store, err := state.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	wt, _, ok := store.Find("feature/orphan")
	if !ok {
		t.Fatalf("expected adopted worktree in state: %#v", store.Worktrees)
	}
	if wt.Branch != "feature/orphan" || wt.Path != filepath.Join(config.WorktreeDir, "feature", "orphan") {
		t.Fatalf("unexpected adopted worktree: %#v", wt)
	}
}

func TestDoctorFixMarksExistingCreatingWorktreeActive(t *testing.T) {
	root := initGitRepo(t)
	if err := config.Ensure(root); err != nil {
		t.Fatal(err)
	}
	now := state.NewStore(root)
	now.Worktrees = append(now.Worktrees, state.Worktree{
		ID:     "feature/creating",
		Name:   "feature/creating",
		Branch: "feature/creating",
		Path:   filepath.Join(config.WorktreeDir, "feature", "creating"),
		Status: state.Status{LastKnown: "creating"},
	})
	if err := state.Save(root, now); err != nil {
		t.Fatal(err)
	}
	worktreePath := filepath.Join(root, config.WorktreeDir, "feature", "creating")
	runGit(t, root, "worktree", "add", "-b", "feature/creating", worktreePath, "HEAD")
	t.Chdir(root)

	result, err := New().Doctor(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	if !hasCheck(result, "worktree feature/creating status cleanup", "marked active") {
		t.Fatalf("expected creating cleanup check, got %#v", result.Checks)
	}
	store, err := state.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	wt, _, ok := store.Find("feature/creating")
	if !ok {
		t.Fatal("expected worktree to remain in state")
	}
	if wt.Status.LastKnown != "active" {
		t.Fatalf("status = %q, want active", wt.Status.LastKnown)
	}
}

func TestReconcileGitWorktreesReturnsGitErrors(t *testing.T) {
	root := t.TempDir()
	store := state.NewStore(root)

	_, _, err := New().reconcileGitWorktrees(context.Background(), root, &store, false)
	if err == nil {
		t.Fatal("expected git worktree list error outside a repository")
	}
}

func hasCheck(result DoctorResult, name, status string) bool {
	for _, check := range result.Checks {
		if check.Name == name && check.Status == status {
			return true
		}
	}
	return false
}

func hasCheckPrefix(result DoctorResult, name, statusPrefix string) bool {
	for _, check := range result.Checks {
		if check.Name == name && strings.HasPrefix(check.Status, statusPrefix) {
			return true
		}
	}
	return false
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, output)
	}
}
