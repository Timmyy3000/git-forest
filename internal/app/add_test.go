package app

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Timmyy3000/git-forest/internal/config"
)

func TestAddFailsBeforeGitWorktreeWhenConfigCannotLoad(t *testing.T) {
	root := initGitRepo(t)
	if err := config.Ensure(root); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(root, config.ConfigPath)
	if err := os.Remove(configPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(configPath, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)

	_, err := New().Add(context.Background(), AddOptions{Name: "bad-config", Agent: "test"})
	if err == nil {
		t.Fatal("expected add to fail when config cannot be loaded")
	}
	out := runGitOutput(t, root, "worktree", "list", "--porcelain")
	if strings.Contains(out, "bad-config") {
		t.Fatalf("worktree should not be created when config load fails:\n%s", out)
	}
}

func TestAddRejectsOptionLikeBaseBeforeGitWorktree(t *testing.T) {
	root := initGitRepo(t)
	t.Chdir(root)

	_, err := New().Add(context.Background(), AddOptions{Name: "bad-base", From: "--git-dir=outside"})
	if err == nil || !strings.Contains(err.Error(), "values beginning with '-'") {
		t.Fatalf("error = %v, want invalid revision", err)
	}
	out := runGitOutput(t, root, "worktree", "list", "--porcelain")
	if strings.Contains(out, "bad-base") {
		t.Fatalf("worktree should not be created for invalid base:\n%s", out)
	}
}

func TestAddRejectsOptionLikeBranchBeforeGitWorktree(t *testing.T) {
	root := initGitRepo(t)
	t.Chdir(root)

	_, err := New().Add(context.Background(), AddOptions{Name: "bad-branch", Branch: "--git-dir=outside"})
	if err == nil || !strings.Contains(err.Error(), "values beginning with '-'") {
		t.Fatalf("error = %v, want invalid revision", err)
	}
	out := runGitOutput(t, root, "worktree", "list", "--porcelain")
	if strings.Contains(out, "bad-branch") {
		t.Fatalf("worktree should not be created for invalid branch:\n%s", out)
	}
}

func initGitRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	initGitRepoAt(t, root)
	return root
}

func initGitRepoAt(t *testing.T, root string) {
	t.Helper()
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "init")
	runGit(t, root, "config", "user.email", "test@example.com")
	runGit(t, root, "config", "user.name", "Forest Test")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", "README.md")
	runGit(t, root, "commit", "-m", "init")
}

func runGitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, output)
	}
	return string(output)
}
