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
	if !hasCheckPrefix(result, "worktree feature/orphan", "untracked by Forest state") {
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

func TestDoctorReconcilesStaleResidualAndStatusDoesNotInspectParent(t *testing.T) {
	root := initGitRepo(t)
	if err := config.Ensure(root); err != nil {
		t.Fatal(err)
	}
	residualPath := filepath.Join(root, config.WorktreeDir, "ft", "stale-residual")
	if err := os.MkdirAll(residualPath, 0o755); err != nil {
		t.Fatal(err)
	}
	store := state.NewStore(root)
	store.Worktrees = append(store.Worktrees, state.Worktree{
		ID:     "ft/stale-residual",
		Name:   "ft/stale-residual",
		Branch: "ft/stale-residual",
		Path:   filepath.Join(config.WorktreeDir, "ft", "stale-residual"),
	})
	if err := state.Save(root, store); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)

	doctor, err := New().Doctor(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if !hasCheckPrefix(doctor, "worktree ft/stale-residual", "stale residual:") {
		t.Fatalf("expected stale residual diagnostic, got %#v", doctor.Checks)
	}
	status, err := New().Status(context.Background(), StatusOptions{})
	if err != nil {
		t.Fatal(err)
	}
	view := status.Worktrees[0]
	if view.Integration != "unknown" || view.Dirty || !view.ChecksIncomplete {
		t.Fatalf("invalid worktree status = %+v, want unknown and incomplete without parent-repo dirty state", view)
	}
	if !strings.Contains(view.CheckError, ".git marker is missing") {
		t.Fatalf("status error = %q, want missing marker diagnostic", view.CheckError)
	}

	doctor, err = New().Doctor(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	if !hasCheck(doctor, "worktree ft/stale-residual residual cleanup", "removed") {
		t.Fatalf("expected residual cleanup, got %#v", doctor.Checks)
	}
	if _, err := os.Stat(residualPath); !os.IsNotExist(err) {
		t.Fatalf("residual folder still exists, stat err = %v", err)
	}
	store, err = state.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, ok := store.Find("ft/stale-residual"); ok {
		t.Fatal("stale residual should be removed from Forest state")
	}

	second, err := New().Doctor(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	if hasCheckPrefix(second, "worktree ft/stale-residual residual cleanup", "") || hasCheckPrefix(second, "state path cleanup", "") {
		t.Fatalf("second doctor --fix made additional stale cleanup changes: %#v", second.Checks)
	}
}

func TestDoctorFixRemovesStaleStateAndPrunableGitMetadata(t *testing.T) {
	root := initGitRepo(t)
	if err := config.Ensure(root); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, config.WorktreeDir, "feature", "prunable")
	runGit(t, root, "worktree", "add", "-b", "feature/prunable", path, "HEAD")
	if err := os.RemoveAll(path); err != nil {
		t.Fatal(err)
	}
	store := state.NewStore(root)
	store.Worktrees = append(store.Worktrees, state.Worktree{
		ID:     "feature/prunable",
		Name:   "feature/prunable",
		Branch: "feature/prunable",
		Path:   filepath.Join(config.WorktreeDir, "feature", "prunable"),
	})
	if err := state.Save(root, store); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)

	doctor, err := New().Doctor(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if !hasCheckPrefix(doctor, "worktree feature/prunable", "stale state:") {
		t.Fatalf("expected stale state diagnostic, got %#v", doctor.Checks)
	}
	if !hasCheckPrefix(doctor, "git worktree feature/prunable", "prunable Git metadata:") {
		t.Fatalf("expected prunable metadata diagnostic, got %#v", doctor.Checks)
	}

	if _, err := New().Doctor(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	store, err = state.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, ok := store.Find("feature/prunable"); ok {
		t.Fatal("stale state should be removed")
	}
	worktrees := runGitOutput(t, root, "worktree", "list", "--porcelain")
	if strings.Contains(worktrees, filepath.ToSlash(path)) {
		t.Fatalf("prunable Git metadata remains:\n%s", worktrees)
	}
}

func TestDoctorFixNeverRemovesPathOutsideForestWorktrees(t *testing.T) {
	root := initGitRepo(t)
	if err := config.Ensure(root); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(root, "outside-residual")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	store := state.NewStore(root)
	store.Worktrees = append(store.Worktrees, state.Worktree{
		ID:   "unsafe",
		Name: "unsafe",
		Path: filepath.Join(config.WorktreeDir, "..", "..", "outside-residual"),
	})
	if err := state.Save(root, store); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	if _, err := New().Doctor(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatalf("doctor removed or changed outside path: %v", err)
	}
}

func TestDoctorFixLeavesValidDirtyWorktreeUntouched(t *testing.T) {
	root := initGitRepo(t)
	runGit(t, root, "branch", "-M", "main")
	config.Ensure(root)
	t.Chdir(root)
	added, err := New().Add(context.Background(), AddOptions{Name: "dirty"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(added.Path, "dirty.txt"), []byte("dirty\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := New().Doctor(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(added.Path, ".git")); err != nil {
		t.Fatalf("valid worktree marker changed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(added.Path, "dirty.txt")); err != nil {
		t.Fatalf("valid dirty worktree changed: %v", err)
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
