package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/Timmyy3000/git-forest/internal/pathutil"
)

func TestIntegrationClassifiesLiveBranchState(t *testing.T) {
	t.Run("unmerged", func(t *testing.T) {
		root := initRepo(t)
		runGit(t, root, "checkout", "-b", "feature")
		commitFile(t, root, "feature.txt", "feature\n", "feature")

		status, err := Integration(context.Background(), root, "main")
		if err != nil {
			t.Fatal(err)
		}
		if status != "unmerged" {
			t.Fatalf("status = %q, want unmerged", status)
		}
	})

	t.Run("merged", func(t *testing.T) {
		root := initRepo(t)

		status, err := Integration(context.Background(), root, "main")
		if err != nil {
			t.Fatal(err)
		}
		if status != "merged" {
			t.Fatalf("status = %q, want merged", status)
		}
	})

	t.Run("patch equivalent", func(t *testing.T) {
		root := initRepo(t)
		runGit(t, root, "checkout", "-b", "feature")
		commitFile(t, root, "feature.txt", "feature\n", "feature")
		runGit(t, root, "checkout", "main")
		commitFile(t, root, "main.txt", "main\n", "main")
		runGit(t, root, "cherry-pick", "feature")
		runGit(t, root, "checkout", "feature")

		status, err := Integration(context.Background(), root, "main")
		if err != nil {
			t.Fatal(err)
		}
		if status != "patch-equivalent" {
			t.Fatalf("status = %q, want patch-equivalent", status)
		}
	})
}

func TestWorktreesReportsPrunableRegistration(t *testing.T) {
	root := initRepo(t)
	path := filepath.Join(root, "missing")
	runGit(t, root, "worktree", "add", "-b", "feature/prunable", path, "HEAD")
	if err := os.RemoveAll(path); err != nil {
		t.Fatal(err)
	}

	worktrees, err := Worktrees(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	wantPath, err := pathutil.NormalizePath(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, worktree := range worktrees {
		gotPath, err := pathutil.NormalizePath(worktree.Path)
		if err != nil {
			t.Fatal(err)
		}
		if gotPath == wantPath {
			if !worktree.Prunable {
				t.Fatal("expected missing worktree registration to be prunable")
			}
			if worktree.PrunableReason == "" {
				t.Fatal("expected prunable reason")
			}
			return
		}
	}
	t.Fatalf("missing worktree path %q not returned", path)
}

func TestIntegrationReportsUninspectableWorktree(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	status, err := Integration(context.Background(), missing, "main")
	if err == nil {
		t.Fatal("expected an inspection error")
	}
	if status != "unknown" {
		t.Fatalf("status = %q, want unknown", status)
	}
}

func TestIntegrationReportsUnresolvedBase(t *testing.T) {
	root := initRepo(t)
	status, err := Integration(context.Background(), root, "missing-base")
	if err == nil {
		t.Fatal("expected an inspection error")
	}
	if status != "unknown" {
		t.Fatalf("status = %q, want unknown", status)
	}
}

func TestIntegrationRejectsOptionLikeBase(t *testing.T) {
	root := initRepo(t)
	status, err := Integration(context.Background(), root, "--git-dir=outside")
	if err == nil {
		t.Fatal("expected an inspection error")
	}
	if status != "unknown" {
		t.Fatalf("status = %q, want unknown", status)
	}
}

func TestAheadBehindWithErrorRejectsInvalidBase(t *testing.T) {
	root := initRepo(t)
	for _, base := range []string{"", "--git-dir=outside"} {
		t.Run(base, func(t *testing.T) {
			_, _, err := AheadBehindWithError(context.Background(), root, base)
			if err == nil {
				t.Fatal("expected an invalid revision error")
			}
		})
	}
}

func TestIsAncestorRejectsOptionLikeRevision(t *testing.T) {
	root := initRepo(t)
	_, err := IsAncestor(context.Background(), root, "HEAD", "--git-dir=outside")
	if err == nil {
		t.Fatal("expected an invalid revision error")
	}
}

func initRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	runGit(t, root, "init")
	runGit(t, root, "branch", "-M", "main")
	runGit(t, root, "config", "user.email", "test@example.com")
	runGit(t, root, "config", "user.name", "Forest Test")
	commitFile(t, root, "README.md", "init\n", "init")
	return root
}

func commitFile(t *testing.T, root, name, contents, message string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, name), []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", name)
	runGit(t, root, "commit", "-m", message)
}

func runGit(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, output)
	}
}
