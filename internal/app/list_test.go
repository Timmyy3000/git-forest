package app

import (
	"context"
	"testing"
)

func TestListRunsGitChecksAndPreservesOrder(t *testing.T) {
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

	result, err := application.List(context.Background(), ListOptions{})
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
		if wt.ChecksSkipped {
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
	if wt.Name != "speedy" || wt.Agent != "test" {
		t.Fatalf("state fields should still be populated, got %+v", wt)
	}
}
