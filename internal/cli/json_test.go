package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os/exec"
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
