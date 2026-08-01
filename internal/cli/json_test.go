package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Timmyy3000/git-forest/internal/app"
)

func TestJSONFlagWorksForAgentContractCommands(t *testing.T) {
	rootDir := t.TempDir()
	runGit(t, rootDir, "init")
	runGit(t, rootDir, "config", "user.email", "test@example.com")
	runGit(t, rootDir, "config", "user.name", "Forest Test")
	runGit(t, rootDir, "commit", "--allow-empty", "-m", "init")
	runGit(t, rootDir, "branch", "-M", "main")
	t.Chdir(rootDir)

	for _, args := range [][]string{
		{"init", "--json"},
		{"list", "--json"},
		{"status", "--json"},
		{"doctor", "--json"},
		{"agents", "--json"},
	} {
		t.Run(args[0], func(t *testing.T) {
			out, err := executeTestCommand(args...)
			if err != nil {
				t.Fatalf("command failed: %v\n%s", err, out)
			}
			var decoded map[string]any
			if err := json.Unmarshal([]byte(out), &decoded); err != nil {
				t.Fatalf("expected JSON output, got %q: %v", out, err)
			}
		})
	}
}

func TestCloseMergedRejectsMultipleNames(t *testing.T) {
	_, err := executeTestCommand("close", "--merged", "first", "second")
	if err == nil || !strings.Contains(err.Error(), "at most one worktree name") {
		t.Fatalf("error = %v, want multiple-name validation", err)
	}
}

func TestListJSONMarksIntegrationOnlyChecks(t *testing.T) {
	rootDir := t.TempDir()
	runGit(t, rootDir, "init")
	runGit(t, rootDir, "config", "user.email", "test@example.com")
	runGit(t, rootDir, "config", "user.name", "Forest Test")
	runGit(t, rootDir, "commit", "--allow-empty", "-m", "init")
	runGit(t, rootDir, "branch", "-M", "main")
	t.Chdir(rootDir)
	application := app.New()
	if _, err := application.Init(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := application.Add(context.Background(), app.AddOptions{Name: "live", Agent: "test", From: "main"}); err != nil {
		t.Fatal(err)
	}

	out, err := executeTestCommand("list", "--json")
	if err != nil {
		t.Fatalf("list failed: %v\n%s", err, out)
	}
	var decoded struct {
		Worktrees []map[string]any `json:"worktrees"`
	}
	if err := json.Unmarshal([]byte(out), &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded.Worktrees) != 1 {
		t.Fatalf("worktrees = %d, want 1", len(decoded.Worktrees))
	}
	worktree := decoded.Worktrees[0]
	if worktree["integration"] == "" {
		t.Fatal("expected live integration in default list JSON")
	}
	if worktree["detailsSkipped"] != true {
		t.Fatalf("detailsSkipped = %v, want true", worktree["detailsSkipped"])
	}
	if _, ok := worktree["checksSkipped"]; ok {
		t.Fatalf("checksSkipped should be absent from integration-only list JSON: %+v", worktree)
	}
	if worktree["baseRef"] != "refs/heads/main" || worktree["comparisonSource"] != "local" {
		t.Fatalf("comparison metadata = baseRef=%v source=%v, want refs/heads/main/local", worktree["baseRef"], worktree["comparisonSource"])
	}
}

func TestListRecursiveJSONUsesRepositoryRelativePaths(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "root")
	child := filepath.Join(parent, "child")
	for _, repository := range []struct {
		path string
		name string
	}{
		{path: root, name: "root-worktree"},
		{path: child, name: "child-worktree"},
	} {
		initForestRepo(t, repository.path)
		t.Chdir(repository.path)
		application := app.New()
		if _, err := application.Init(context.Background()); err != nil {
			t.Fatal(err)
		}
		if _, err := application.Add(context.Background(), app.AddOptions{Name: repository.name}); err != nil {
			t.Fatal(err)
		}
	}

	t.Chdir(parent)
	out, err := executeTestCommand("list", "-r", "--json")
	if err != nil {
		t.Fatalf("recursive list failed: %v\n%s", err, out)
	}
	var decoded struct {
		Repositories []struct {
			Path      string           `json:"path"`
			Worktrees []map[string]any `json:"worktrees"`
		} `json:"repositories"`
	}
	if err := json.Unmarshal([]byte(out), &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded.Repositories) != 2 {
		t.Fatalf("repositories = %d, want 2", len(decoded.Repositories))
	}
	if decoded.Repositories[0].Path != "./child" || decoded.Repositories[1].Path != "./root" {
		t.Fatalf("repository paths = %+v", decoded.Repositories)
	}
	expectedNames := map[string]string{"./child": "child-worktree", "./root": "root-worktree"}
	for _, repository := range decoded.Repositories {
		if len(repository.Worktrees) != 1 {
			t.Fatalf("repository %q worktrees = %+v, want one worktree", repository.Path, repository.Worktrees)
		}
		name, ok := repository.Worktrees[0]["name"].(string)
		if !ok || name != expectedNames[repository.Path] {
			t.Fatalf("repository %q worktree name = %v, want %q", repository.Path, repository.Worktrees[0]["name"], expectedNames[repository.Path])
		}
	}
}

func executeTestCommand(args ...string) (string, error) {
	outputJSON = false
	command := newRootCommand(app.New())
	var out bytes.Buffer
	command.SetOut(&out)
	command.SetErr(&out)
	command.SetArgs(args)
	err := command.Execute()
	return out.String(), err
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, output)
	}
}

func initForestRepo(t *testing.T, root string) {
	t.Helper()
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "init")
	runGit(t, root, "config", "user.email", "test@example.com")
	runGit(t, root, "config", "user.name", "Forest Test")
	runGit(t, root, "commit", "--allow-empty", "-m", "init")
	runGit(t, root, "branch", "-M", "main")
}
