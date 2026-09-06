package app

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
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

func TestRecoveryPreservesDanglingWorkspaceJunctions(t *testing.T) {
	root := initGitRepo(t)
	runGit(t, root, "branch", "-M", "main")
	t.Chdir(root)
	application := New()
	added, err := application.Add(context.Background(), AddOptions{Name: "junction-residual"})
	if err != nil {
		t.Fatal(err)
	}
	// Model an interrupted removal using only a disposable, unregistered fixture.
	runGit(t, root, "worktree", "remove", added.Path)
	modules := filepath.Join(added.Path, "node_modules")
	if err := os.MkdirAll(modules, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"mobile", "web"} {
		target := filepath.Join(added.Path, "apps", name)
		if err := os.MkdirAll(target, 0o755); err != nil {
			t.Fatal(err)
		}
		link := filepath.Join(modules, name)
		// Directory junctions do not require Windows symlink privileges. Quote
		// both fixture paths explicitly for cmd, including paths with spaces.
		cmd := exec.Command("cmd")
		cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: `cmd /d /c mklink /J "` + link + `" "` + target + `"`}
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("create fixture junction: %v: %s", err, output)
		}
		if err := os.Remove(target); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(link); !os.IsNotExist(err) {
			t.Fatalf("junction should be dangling: %v", err)
		}
	}
	files := []string{"evidence.md", "config.local", "one.txt", "two.txt", "three.txt", "four.txt"}
	for _, name := range files {
		if err := os.WriteFile(filepath.Join(added.Path, name), []byte("preserve fixture evidence\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	result, err := application.Close(context.Background(), CloseOptions{
		Name: added.Name, Yes: true, DeleteBranch: true, IncludeDirty: true, IncludeUnmerged: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Closed) != 0 || len(result.Skipped) != 1 || !strings.Contains(result.Skipped[0].Reason, "not empty") {
		t.Fatalf("close must preserve junction residue even with include flags: %+v", result)
	}
	if _, err := application.Doctor(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	for _, name := range files {
		content, err := os.ReadFile(filepath.Join(added.Path, name))
		if err != nil || string(content) != "preserve fixture evidence\n" {
			t.Fatalf("residual file %s changed: %q, %v", name, content, err)
		}
	}
	for _, name := range []string{"mobile", "web"} {
		link := filepath.Join(modules, name)
		if _, err := os.Lstat(link); err != nil {
			t.Fatalf("dangling junction %s was lost: %v", name, err)
		}
		linkName, err := windows.UTF16PtrFromString(link)
		if err != nil {
			t.Fatal(err)
		}
		attributes, err := windows.GetFileAttributes(linkName)
		wantAttributes := uint32(windows.FILE_ATTRIBUTE_DIRECTORY | windows.FILE_ATTRIBUTE_REPARSE_POINT)
		if err != nil || attributes&wantAttributes != wantAttributes {
			t.Fatalf("junction %s no longer has directory/reparse attributes: %x, %v", name, attributes, err)
		}
		if _, err := os.Stat(link); !os.IsNotExist(err) {
			t.Fatalf("recovery recreated junction target %s: %v", name, err)
		}
	}
	store, err := state.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, ok := store.Find(added.Name); !ok || !git.BranchExists(context.Background(), root, added.Branch) {
		t.Fatal("recovery lost residual record or branch")
	}
	registry := loadGitWorktreeRegistry(context.Background(), root)
	if registry.LoadErr != nil {
		t.Fatal(registry.LoadErr)
	}
	if _, registered := registry.ByPath[worktreePathKey(added.Path)]; registered {
		t.Fatal("residual was unexpectedly registered with Git")
	}
}
