package pathutil

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestFromNameUsesForestBranch(t *testing.T) {
	mapping, err := FromName(".forest/worktrees", "fix-login")
	if err != nil {
		t.Fatal(err)
	}
	if mapping.Identity != "fix-login" {
		t.Fatalf("identity = %q", mapping.Identity)
	}
	if mapping.Branch != "forest/fix-login" {
		t.Fatalf("branch = %q", mapping.Branch)
	}
}

func TestFromBranchUsesBranchAsIdentity(t *testing.T) {
	mapping, err := FromBranch(".forest/worktrees", "feat/login-copy")
	if err != nil {
		t.Fatal(err)
	}
	if mapping.Identity != "feat/login-copy" {
		t.Fatalf("identity = %q", mapping.Identity)
	}
	if mapping.Branch != "feat/login-copy" {
		t.Fatalf("branch = %q", mapping.Branch)
	}
}

func TestNormalizePathCleansAndMakesAbsolute(t *testing.T) {
	path, err := NormalizePath(filepath.Join(".", "forest", "..", "worktrees", "feature", "nested"))
	if err != nil {
		t.Fatal(err)
	}
	want, err := NormalizePath(filepath.Join("worktrees", "feature", "nested"))
	if err != nil {
		t.Fatal(err)
	}
	if path != want {
		t.Fatalf("normalized path = %q, want %q", path, want)
	}
}

func TestNormalizePathFoldsCaseOnWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("case folding applies to Windows paths")
	}
	path := filepath.Join(t.TempDir(), "Forest", "Worktree")
	lower, err := NormalizePath(strings.ToLower(path))
	if err != nil {
		t.Fatal(err)
	}
	upper, err := NormalizePath(strings.ToUpper(path))
	if err != nil {
		t.Fatal(err)
	}
	if lower != upper {
		t.Fatalf("case-normalized paths differ: %q != %q", lower, upper)
	}
}

func TestNormalizePathPreservesDistinctCaseOnCaseSensitiveDarwin(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("case-sensitive volume behavior applies to Darwin paths")
	}
	root := t.TempDir()
	upper := filepath.Join(root, "Foo")
	lower := filepath.Join(root, "foo")
	if err := os.MkdirAll(upper, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(lower, 0o755); err != nil {
		t.Skipf("filesystem is case-insensitive: %v", err)
	}
	first, err := NormalizePath(upper)
	if err != nil {
		t.Fatal(err)
	}
	second, err := NormalizePath(lower)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatalf("distinct case-sensitive paths were conflated: %q", first)
	}
}

func TestValidateIdentityRejectsTraversal(t *testing.T) {
	for _, input := range []string{"../x", "feat/../../x", "/tmp/x", "feat//x"} {
		if err := ValidateIdentity(input); err == nil {
			t.Fatalf("expected %q to be invalid", input)
		}
	}
}

func TestCheckCollisionRejectsExactAndPrefixCollisions(t *testing.T) {
	existing := []string{
		".forest/worktrees/fix-login",
		".forest/worktrees/feat/login-copy",
	}
	for _, candidate := range []string{
		".forest/worktrees/fix-login",
		".forest/worktrees/feat",
		".forest/worktrees/feat/login-copy/extra",
	} {
		if err := CheckCollision(candidate, existing); err == nil {
			t.Fatalf("expected collision for %s", candidate)
		}
	}
}

func TestContains(t *testing.T) {
	root := t.TempDir()
	parent := filepath.Join(root, "worktrees", "fix-login")
	child := filepath.Join(parent, "sub", "dir")
	sibling := filepath.Join(root, "worktrees", "other")
	for _, dir := range []string{child, sibling} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	ok, err := Contains(parent, child)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected child path to be contained")
	}
	ok, err = Contains(parent, sibling)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected sibling path not to be contained")
	}
}

func TestContainsFoldsCaseOnWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("case folding applies to Windows paths")
	}
	root := t.TempDir()
	parent := filepath.Join(root, "Fix-Login")
	child := filepath.Join(parent, "sub")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatal(err)
	}
	ok, err := Contains(filepath.Join(root, "fix-login"), child)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected differently cased child to be contained")
	}
}

func TestContainsResolvesSymlinks(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "real")
	if err := os.MkdirAll(filepath.Join(target, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "alias")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable on this host: %v", err)
	}
	ok, err := Contains(target, filepath.Join(link, "sub"))
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected symlinked child to be contained in real parent")
	}
	ok, err = Contains(link, filepath.Join(target, "sub"))
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected real child to be contained in symlinked parent")
	}
}

func TestContainsRejectsParentTraversal(t *testing.T) {
	root := t.TempDir()
	parent := filepath.Join(root, "worktrees", "fix-login")
	escape := filepath.Join(root, "worktrees", "escape")
	for _, dir := range []string{parent, escape} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	traversal := filepath.Join(parent, "..", "escape")
	ok, err := Contains(parent, traversal)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected traversal path outside parent to be rejected")
	}
}
