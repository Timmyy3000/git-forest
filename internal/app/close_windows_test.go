package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Timmyy3000/git-forest/internal/config"
	"github.com/Timmyy3000/git-forest/internal/git"
	"github.com/Timmyy3000/git-forest/internal/state"
	"golang.org/x/sys/windows"
)

func TestCloseReportsWindowsPartialRemoval(t *testing.T) {
	root := initGitRepo(t)
	runGit(t, root, "branch", "-M", "main")
	t.Chdir(root)
	added, err := New().Add(context.Background(), AddOptions{Name: "locked"})
	if err != nil {
		t.Fatal(err)
	}
	lockedPath := filepath.Join(added.Path, "README.md")
	want, err := os.ReadFile(lockedPath)
	if err != nil {
		t.Fatal(err)
	}
	name, err := windows.UTF16PtrFromString(lockedPath)
	if err != nil {
		t.Fatal(err)
	}
	// Permit Git to read the clean file, but deny deletion while the handle is open.
	handle, err := windows.CreateFile(name, windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(handle)
	result, err := New().Close(context.Background(), CloseOptions{Name: added.Name, Yes: true, DeleteBranch: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Closed) != 0 || len(result.Skipped) != 1 || !strings.Contains(result.Skipped[0].Reason, "path still exists") {
		t.Fatalf("partial close = %+v", result)
	}
	content, err := os.ReadFile(lockedPath)
	if err != nil || string(content) != string(want) {
		t.Fatalf("locked content = %q, %v", content, err)
	}
	if !git.BranchExists(context.Background(), root, added.Branch) {
		t.Fatal("partial close deleted branch")
	}
	store, err := state.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, ok := store.Find(added.Name); !ok {
		t.Fatal("partial close lost record")
	}
	events, err := os.ReadFile(filepath.Join(root, config.StateDir, "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(events), `"type":"closed"`) {
		t.Fatal("partial close emitted closed event")
	}
}
