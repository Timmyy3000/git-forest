package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Timmyy3000/git-forest/internal/app"
)

func TestRenderStatusKeepsNoteAndDiagnostic(t *testing.T) {
	var out bytes.Buffer
	err := RenderStatus(&out, app.ListResult{Worktrees: []app.WorktreeView{{
		Name:             "cancelled",
		Integration:      "merged",
		Note:             "waiting on CI",
		CheckError:       "integration: context canceled",
		ChecksIncomplete: true,
	}}})
	if err != nil {
		t.Fatal(err)
	}
	rendered := out.String()
	for _, want := range []string{"waiting on CI", "context canceled", "merged (partial)"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("rendered status missing %q:\n%s", want, rendered)
		}
	}
}

func TestRenderStatusMarksFailedIntegrationAsPartial(t *testing.T) {
	var out bytes.Buffer
	err := RenderStatus(&out, app.ListResult{Worktrees: []app.WorktreeView{{
		Name:             "cancelled",
		Integration:      "unknown",
		IntegrationError: "context canceled",
		ChecksIncomplete: true,
	}}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "unknown: context canceled (partial)") {
		t.Fatalf("rendered status missing partial integration indicator:\n%s", out.String())
	}
}

func TestRenderRecursiveListUsesCompactRepositoryGroups(t *testing.T) {
	var out bytes.Buffer
	err := renderRecursiveList(&out, app.RecursiveListResult{Repositories: []app.RepositoryList{
		{Path: ".", Worktrees: []app.WorktreeView{{Name: "root-worktree", Integration: "unmerged", Agent: "ana", Phase: "working"}}},
		{Path: "./child", Worktrees: []app.WorktreeView{{Name: "child-worktree", Integration: "merged", Agent: "kai", Phase: "review"}}},
	}}, 80)
	if err != nil {
		t.Fatal(err)
	}
	rendered := out.String()
	for _, want := range []string{".\n", "./child", "root-worktree", "child-worktree"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("rendered recursive list missing %q:\n%s", want, rendered)
		}
	}
}

func TestRenderRecursiveListShowsEmptyRepositoryInWideTable(t *testing.T) {
	var out bytes.Buffer
	err := renderRecursiveList(&out, app.RecursiveListResult{Repositories: []app.RepositoryList{{Path: "./empty"}}}, 120)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "REPOSITORY") || !strings.Contains(out.String(), "(no managed worktrees)") {
		t.Fatalf("rendered wide recursive list:\n%s", out.String())
	}
}
