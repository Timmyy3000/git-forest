package app

import (
	"context"
	"errors"
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

func TestListRecursiveDiscoversForestRepositories(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "root")
	child := filepath.Join(parent, "child")
	grandchild := filepath.Join(child, "nested")
	initGitRepoAt(t, root)
	initGitRepoAt(t, child)
	initGitRepoAt(t, grandchild)

	application := New()
	t.Chdir(root)
	if _, err := application.Init(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := application.Add(context.Background(), AddOptions{Name: "root-worktree"}); err != nil {
		t.Fatal(err)
	}
	t.Chdir(child)
	if _, err := application.Init(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := application.Add(context.Background(), AddOptions{Name: "child-worktree"}); err != nil {
		t.Fatal(err)
	}
	t.Chdir(grandchild)
	if _, err := application.Init(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := application.Add(context.Background(), AddOptions{Name: "nested-worktree"}); err != nil {
		t.Fatal(err)
	}

	t.Chdir(parent)
	result, err := application.ListRecursive(context.Background(), ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Repositories) != 2 {
		t.Fatalf("repositories = %d, want 2", len(result.Repositories))
	}
	for i, want := range []struct {
		path string
		name string
	}{
		{path: "./child", name: "child-worktree"},
		{path: "./root", name: "root-worktree"},
	} {
		repository := result.Repositories[i]
		if repository.Path != want.path {
			t.Fatalf("repository %d path = %q, want %q", i, repository.Path, want.path)
		}
		if len(repository.Worktrees) != 1 || repository.Worktrees[0].Name != want.name {
			t.Fatalf("repository %q worktrees = %+v", repository.Path, repository.Worktrees)
		}
	}
}

func TestListRecursiveIncludesStartingRepositoryOnce(t *testing.T) {
	root := initGitRepo(t)
	application := New()
	t.Chdir(root)
	if _, err := application.Init(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := application.Add(context.Background(), AddOptions{Name: "managed"}); err != nil {
		t.Fatal(err)
	}

	result, err := application.ListRecursive(context.Background(), ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Repositories) != 1 {
		t.Fatalf("repositories = %d, want 1", len(result.Repositories))
	}
	repository := result.Repositories[0]
	if repository.Path != "." {
		t.Fatalf("repository path = %q, want .", repository.Path)
	}
	if len(repository.Worktrees) != 1 || repository.Worktrees[0].Name != "managed" {
		t.Fatalf("repository worktrees = %+v", repository.Worktrees)
	}
}

func TestListRecursiveKeepsHealthyRepositoriesWhenOneStateFails(t *testing.T) {
	parent := t.TempDir()
	healthy := filepath.Join(parent, "healthy")
	broken := filepath.Join(parent, "broken")
	for _, root := range []string{healthy, broken} {
		initGitRepoAt(t, root)
		t.Chdir(root)
		if _, err := New().Init(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(broken, ".forest", "state", "worktrees.json"), []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Chdir(parent)
	result, err := New().ListRecursive(context.Background(), ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Repositories) != 2 {
		t.Fatalf("repositories = %d, want 2", len(result.Repositories))
	}
	if result.Repositories[0].Path != "./broken" || result.Repositories[0].Error == "" {
		t.Fatalf("broken repository = %+v", result.Repositories[0])
	}
	if result.Repositories[1].Path != "./healthy" || result.Repositories[1].Error != "" {
		t.Fatalf("healthy repository = %+v", result.Repositories[1])
	}
}

func TestListRecursiveWarnsWhenForestMarkerIsNotDirectory(t *testing.T) {
	parent := t.TempDir()
	child := filepath.Join(parent, "invalid")
	if err := os.Mkdir(child, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(child, ".forest"), []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Chdir(parent)
	result, err := New().ListRecursive(context.Background(), ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Repositories) != 0 {
		t.Fatalf("repositories = %+v, want none", result.Repositories)
	}
	if len(result.Warnings) != 1 || !strings.Contains(result.Warnings[0], ".forest is not a directory") {
		t.Fatalf("warnings = %+v", result.Warnings)
	}
}

func TestListRecursiveHonorsCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := New().ListRecursive(ctx, ListOptions{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ListRecursive error = %v, want context canceled", err)
	}
}

func TestListRecursiveFastSkipsGitChecks(t *testing.T) {
	root := initGitRepo(t)
	t.Chdir(root)
	application := New()
	if _, err := application.Init(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := application.Add(context.Background(), AddOptions{Name: "speedy"}); err != nil {
		t.Fatal(err)
	}

	result, err := application.ListRecursive(context.Background(), ListOptions{Fast: true})
	if err != nil {
		t.Fatal(err)
	}
	worktree := result.Repositories[0].Worktrees[0]
	if !worktree.ChecksSkipped || worktree.DetailsSkipped || worktree.Integration != "unknown" {
		t.Fatalf("fast recursive worktree = %+v", worktree)
	}
}

func TestSamePathResolvesSymlinks(t *testing.T) {
	realPath := t.TempDir()
	linkPath := filepath.Join(t.TempDir(), "linked")
	if err := os.Symlink(realPath, linkPath); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if !samePath(realPath, linkPath) {
		t.Fatalf("samePath(%q, %q) = false, want true", realPath, linkPath)
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
	if !wt.ChecksIncomplete {
		t.Fatal("expected failed integration-only check to be marked incomplete")
	}
}

func TestCloseRemovesMissingWorktreeState(t *testing.T) {
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

	result, err := application.Close(context.Background(), CloseOptions{Name: "missing", Yes: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Closed) != 1 || len(result.Skipped) != 0 {
		t.Fatalf("close result = %+v, want stale worktree to close", result)
	}
	if _, err := os.Stat(added.Path); !os.IsNotExist(err) {
		t.Fatalf("stale worktree folder should be removed, stat err = %v", err)
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

func TestStatusDiffRejectsFastMode(t *testing.T) {
	_, err := New().Status(context.Background(), StatusOptions{Name: "feature", Diff: true, Fast: true})
	if err == nil || !strings.Contains(err.Error(), "cannot be used with --fast") {
		t.Fatalf("error = %v, want fast/diff incompatibility", err)
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
	if !checks.dirty || checks.ahead != -1 || checks.behind != -1 {
		t.Fatalf("cancelled checks = dirty=%t ahead=%d behind=%d, want conservative sentinels", checks.dirty, checks.ahead, checks.behind)
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
