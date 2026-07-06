package cli

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"testing"

	"github.com/Timmyy3000/git-forest/internal/app"
)

func TestJSONFlagWorksForAgentContractCommands(t *testing.T) {
	rootDir := t.TempDir()
	runGit(t, rootDir, "init")
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
