package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
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

	t.Run("normal merge", func(t *testing.T) {
		root := initRepo(t)
		runGit(t, root, "checkout", "-b", "feature")
		commitFile(t, root, "feature.txt", "feature\n", "feature")
		runGit(t, root, "checkout", "main")
		runGit(t, root, "merge", "--no-ff", "feature", "-m", "merge feature")
		runGit(t, root, "checkout", "feature")

		status, err := Integration(context.Background(), root, "main")
		if err != nil {
			t.Fatal(err)
		}
		if status != "merged" {
			t.Fatalf("status = %q, want merged", status)
		}
	})
}

func TestIntegrationRefUsesNamedHeadFromRepositoryRoot(t *testing.T) {
	root := initRepo(t)
	runGit(t, root, "checkout", "-b", "feature")
	commitFile(t, root, "feature.txt", "feature\n", "feature")
	runGit(t, root, "checkout", "main")

	status, err := IntegrationRef(context.Background(), root, "main", "feature")
	if err != nil {
		t.Fatal(err)
	}
	if status != "unmerged" {
		t.Fatalf("status = %q, want unmerged", status)
	}
}

func TestAheadBehindRefUsesNamedHeadFromRepositoryRoot(t *testing.T) {
	root := initRepo(t)
	runGit(t, root, "checkout", "-b", "feature")
	commitFile(t, root, "feature.txt", "feature\n", "feature")
	runGit(t, root, "checkout", "main")

	ahead, behind, err := AheadBehindRefWithError(context.Background(), root, "main", "feature")
	if err != nil {
		t.Fatal(err)
	}
	if ahead != 1 || behind != 0 {
		t.Fatalf("ahead/behind = %d/%d, want 1/0", ahead, behind)
	}
}

func TestResolveComparisonRefPrefersOriginAndHandlesQualifiedRefs(t *testing.T) {
	root := initRepo(t)
	runGit(t, root, "update-ref", "refs/remotes/origin/main", "HEAD")

	resolved, err := ResolveComparisonRef(context.Background(), root, "main")
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Ref != "origin/main" || resolved.Source != "remote-tracking" {
		t.Fatalf("resolved = %+v, want origin/main from remote-tracking", resolved)
	}
	if resolved.OID != gitOutput(t, root, "rev-parse", resolved.Ref) {
		t.Fatalf("resolved OID = %q, want current origin/main tip", resolved.OID)
	}

	resolved, err = ResolveComparisonRef(context.Background(), root, "origin/main")
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Ref != "origin/main" || resolved.Source != "remote-tracking" {
		t.Fatalf("qualified resolved = %+v, want origin/main from remote-tracking", resolved)
	}

	runGit(t, root, "update-ref", "-d", "refs/remotes/origin/main")
	runGit(t, root, "commit", "--allow-empty", "-m", "branch tip")
	runGit(t, root, "tag", "main", "HEAD^")
	resolved, err = ResolveComparisonRef(context.Background(), root, "main")
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Ref != "refs/heads/main" || resolved.Source != "local" {
		t.Fatalf("fallback resolved = %+v, want refs/heads/main from local", resolved)
	}
	branchTip := gitOutput(t, root, "rev-parse", "refs/heads/main")
	if resolved.OID != branchTip {
		t.Fatalf("resolved OID = %q, want branch tip %q", resolved.OID, branchTip)
	}
	resolvedTip := gitOutput(t, root, "rev-parse", resolved.Ref)
	if resolvedTip != branchTip {
		t.Fatalf("resolved tip = %q, want branch tip %q", resolvedTip, branchTip)
	}
}

func TestResolveComparisonRefReportsMissingBase(t *testing.T) {
	root := initRepo(t)
	if _, err := ResolveComparisonRef(context.Background(), root, "missing"); err == nil {
		t.Fatal("expected missing comparison ref to fail")
	}
}

func TestIntegrationRecognizesMultiCommitSquashAgainstRemoteBase(t *testing.T) {
	root := initRepo(t)
	runGit(t, root, "checkout", "-b", "feature")
	commitFile(t, root, "one.txt", "one\n", "one")
	commitFile(t, root, "two.txt", "two\n", "two")

	runGit(t, root, "checkout", "main")
	runGit(t, root, "checkout", "-b", "integration")
	runGit(t, root, "merge", "--squash", "feature")
	runGit(t, root, "commit", "-m", "squash feature")
	target := gitOutput(t, root, "rev-parse", "HEAD")
	runGit(t, root, "checkout", "feature")
	runGit(t, root, "update-ref", "refs/remotes/origin/dev", target)

	resolved, err := ResolveComparisonRef(context.Background(), root, "dev")
	if err != nil {
		t.Fatal(err)
	}
	status, err := Integration(context.Background(), root, resolved.OID)
	if err != nil {
		t.Fatal(err)
	}
	if status != "patch-equivalent" {
		t.Fatalf("status = %q, want patch-equivalent", status)
	}
}

func TestIntegrationKeepsConflictingChangesUnmerged(t *testing.T) {
	root := initRepo(t)
	runGit(t, root, "checkout", "-b", "feature")
	commitFile(t, root, "shared.txt", "feature\n", "feature")
	runGit(t, root, "checkout", "main")
	commitFile(t, root, "shared.txt", "target\n", "target")
	runGit(t, root, "checkout", "feature")

	status, err := Integration(context.Background(), root, "main")
	if err != nil {
		t.Fatal(err)
	}
	if status != "unmerged" {
		t.Fatalf("status = %q, want unmerged", status)
	}
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

func TestValidateRevisionRejectsControlCharacters(t *testing.T) {
	if err := ValidateRevision("main\nfeature"); err == nil {
		t.Fatal("expected control characters to be rejected")
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

func gitOutput(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v failed: %v", args, err)
	}
	return strings.TrimSpace(string(output))
}
