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
