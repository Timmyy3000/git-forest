package app

import (
	"path/filepath"
	"testing"

	"github.com/oluwatimilehin/git-forest/internal/config"
	"github.com/oluwatimilehin/git-forest/internal/pathutil"
)

func TestValidStatePathAcceptsGeneratedWorktreePaths(t *testing.T) {
	fromName, err := pathutil.FromName(config.WorktreeDir, "fix-login")
	if err != nil {
		t.Fatal(err)
	}
	fromBranch, err := pathutil.FromBranch(config.WorktreeDir, "feat/login-copy")
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name string
		path string
	}{
		{name: "from name", path: fromName.RelPath},
		{name: "from branch", path: fromBranch.RelPath},
		{name: "nested", path: filepath.Join(config.WorktreeDir, "chore", "docs")},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if !validStatePath(tt.path) {
				t.Fatalf("expected %s to be valid", tt.path)
			}
		})
	}
}

func TestValidStatePathRejectsUnsafePaths(t *testing.T) {
	for _, path := range []string{"", ".", "../escape", filepath.Join(".forest", "other", "fix-login"), filepath.Join(config.WorktreeDir, "..", "escape")} {
		if validStatePath(path) {
			t.Fatalf("expected %s to be invalid", path)
		}
	}
}
