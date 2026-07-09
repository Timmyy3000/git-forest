package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListDetailedRunsGitChecksAndPreservesOrder(t *testing.T) {
	root := initGitRepo(t)
	runGit(t, root, "branch", "-M", "main")
	t.Chdir(root)
	application := New()

	names := []string{"alpha", "beta", "gamma"}
	for _, name := range names {
		if _, err := application.Add(context.Background(), AddOptions{Name: name, Agent: "test"}); err != nil {
			t.Fatal(err)
		}
	}

	result, err := application.List(context.Background(), ListOptions{Detailed: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Worktrees) != len(names) {
		t.Fatalf("expected %d worktrees, got %d", len(names), len(result.Worktrees))
	}
	for i, wt := range result.Worktrees {
		if wt.Name != names[i] {
			t.Fatalf("worktree %d = %q, want %q (order must match state)", i, wt.Name, names[i])
		}
		if wt.ChecksSkipped || wt.DetailsSkipped {
			t.Fatalf("worktree %q should have git checks, got ChecksSkipped", wt.Name)
		}
		if wt.Integration == "" {
			t.Fatalf("worktree %q missing integration status", wt.Name)
		}
		if wt.Next == "" {
			t.Fatalf("worktree %q missing next action", wt.Name)
		}
	}
}

func TestListDefaultRefreshesIntegrationButSkipsDetails(t *testing.T) {
	root := initGitRepo(t)
	runGit(t, root, "branch", "-M", "main")
	t.Chdir(root)
	application := New()

	if _, err := application.Add(context.Background(), AddOptions{Name: "live", Agent: "test"}); err != nil {
		t.Fatal(err)
	}

	result, err := application.List(context.Background(), ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	wt := result.Worktrees[0]
	if wt.ChecksSkipped || !wt.DetailsSkipped {
		t.Fatalf("list check mode = %+v, want live integration only", wt)
	}
	if wt.Integration != "merged" {
		t.Fatalf("integration = %q, want merged", wt.Integration)
	}
	if wt.Next != "" {
		t.Fatalf("next = %q, want empty when details are skipped", wt.Next)
	}
}

func TestListReportsUninspectableIntegration(t *testing.T) {
	root := initGitRepo(t)
	runGit(t, root, "branch", "-M", "main")
	t.Chdir(root)
	application := New()

	added, err := application.Add(context.Background(), AddOptions{Name: "missing", Agent: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(added.Path); err != nil {
		t.Fatal(err)
	}

	result, err := application.List(context.Background(), ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	wt := result.Worktrees[0]
	if wt.Integration != "unknown" {
		t.Fatalf("integration = %q, want unknown", wt.Integration)
	}
	if wt.IntegrationError == "" {
		t.Fatal("expected integration inspection diagnostic")
	}
}

func TestStatusProvidesDetailedHealthAndNamedDiff(t *testing.T) {
	root := initGitRepo(t)
	runGit(t, root, "branch", "-M", "main")
	t.Chdir(root)
	application := New()

	added, err := application.Add(context.Background(), AddOptions{Name: "inspect", Agent: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(added.Path, "README.md"), []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := application.Status(context.Background(), StatusOptions{Name: "inspect", Diff: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Worktrees) != 1 {
		t.Fatalf("worktrees = %d, want 1", len(result.Worktrees))
	}
	wt := result.Worktrees[0]
	if wt.ChecksSkipped || wt.DetailsSkipped || !wt.Dirty {
		t.Fatalf("status view = %+v, want detailed dirty health", wt)
	}
	if !strings.Contains(result.Diff, "changed") {
		t.Fatalf("diff = %q, want changed contents", result.Diff)
	}
}

func TestStatusDiffRequiresName(t *testing.T) {
	_, err := New().Status(context.Background(), StatusOptions{Diff: true})
	if err == nil || !strings.Contains(err.Error(), "requires a worktree name") {
		t.Fatalf("error = %v, want named diff requirement", err)
	}
}

func TestCollectGitChecksReportsCancelledQueuedChecks(t *testing.T) {
	root := initGitRepo(t)
	runGit(t, root, "branch", "-M", "main")
	for range cap(gitCheckSlots) {
		gitCheckSlots <- struct{}{}
	}
	t.Cleanup(func() {
		for range cap(gitCheckSlots) {
			<-gitCheckSlots
		}
	})

	ctx, cancel := context.WithCancel(context.Background())
	checksCh := make(chan gitChecks, 1)
	go func() { checksCh <- collectGitChecks(ctx, root, "main") }()
	cancel()
	checks := <-checksCh

	if !checks.incomplete {
		t.Fatal("expected cancelled checks to be marked incomplete")
	}
	if checks.integration != "unknown" {
		t.Fatalf("integration = %q, want unknown", checks.integration)
	}
	if !strings.Contains(checks.integrationError, context.Canceled.Error()) {
		t.Fatalf("integration error = %q, want cancellation diagnostic", checks.integrationError)
	}
	if !strings.Contains(checks.checkError, context.Canceled.Error()) {
		t.Fatalf("check error = %q, want cancellation diagnostic", checks.checkError)
	}
}

func TestListFastSkipsGitChecks(t *testing.T) {
	root := initGitRepo(t)
	runGit(t, root, "branch", "-M", "main")
	t.Chdir(root)
	application := New()

	if _, err := application.Add(context.Background(), AddOptions{Name: "speedy", Agent: "test"}); err != nil {
		t.Fatal(err)
	}

	result, err := application.List(context.Background(), ListOptions{Fast: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Worktrees) != 1 {
		t.Fatalf("expected 1 worktree, got %d", len(result.Worktrees))
	}
	wt := result.Worktrees[0]
	if !wt.ChecksSkipped {
		t.Fatal("expected ChecksSkipped in fast mode")
	}
	if wt.Integration != "unknown" {
		t.Fatalf("integration = %q, want unknown", wt.Integration)
	}
	if wt.DetailsSkipped {
		t.Fatal("metadata-only fast mode should use ChecksSkipped instead")
	}
	if wt.Name != "speedy" || wt.Agent != "test" {
		t.Fatalf("state fields should still be populated, got %+v", wt)
	}
}
