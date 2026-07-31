package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/Timmyy3000/git-forest/internal/git"
)

func BenchmarkDiscoverForestRootsDepthOne(b *testing.B) {
	root := b.TempDir()
	for i := range 100 {
		if err := os.Mkdir(filepath.Join(root, fmt.Sprintf("repo-%03d", i)), 0o755); err != nil {
			b.Fatal(err)
		}
	}
	deep := filepath.Join(root, "generated")
	for i := range 50 {
		deep = filepath.Join(deep, fmt.Sprintf("d%02d", i))
		if err := os.MkdirAll(deep, 0o755); err != nil {
			b.Fatal(err)
		}
	}

	b.ReportMetric(102, "candidates/op")
	b.ResetTimer()
	for b.Loop() {
		roots, warnings, err := discoverForestRoots(context.Background(), root)
		if err != nil {
			b.Fatal(err)
		}
		if len(roots) != 0 || len(warnings) != 0 {
			b.Fatalf("roots=%v warnings=%v", roots, warnings)
		}
	}
}

func BenchmarkIntegrationAcrossWorktrees(b *testing.B) {
	root := b.TempDir()
	benchmarkGit(b, root, "init")
	benchmarkGit(b, root, "config", "user.email", "test@example.com")
	benchmarkGit(b, root, "config", "user.name", "Forest Benchmark")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("init\n"), 0o644); err != nil {
		b.Fatal(err)
	}
	benchmarkGit(b, root, "add", "README.md")
	benchmarkGit(b, root, "commit", "-m", "init")
	benchmarkGit(b, root, "branch", "-M", "main")

	const worktreeCount = 5
	paths := make([]string, 0, worktreeCount)
	for i := 0; i < worktreeCount; i++ {
		branch := fmt.Sprintf("benchmark-%d", i)
		path := filepath.Join(root, branch)
		benchmarkGit(b, root, "worktree", "add", "-b", branch, path, "main")
		if err := os.WriteFile(filepath.Join(path, "change.txt"), []byte(fmt.Sprintf("change %d\n", i)), 0o644); err != nil {
			b.Fatal(err)
		}
		benchmarkGit(b, path, "add", "change.txt")
		benchmarkGit(b, path, "commit", "-m", "change")
		paths = append(paths, path)
	}

	b.ResetTimer()
	for b.Loop() {
		for _, path := range paths {
			if status, err := git.Integration(context.Background(), path, "main"); err != nil || status == "" {
				b.Fatalf("integration status=%q err=%v", status, err)
			}
		}
	}
}

func BenchmarkResolveComparisonRef(b *testing.B) {
	root := b.TempDir()
	benchmarkGit(b, root, "init")
	benchmarkGit(b, root, "config", "user.email", "test@example.com")
	benchmarkGit(b, root, "config", "user.name", "Forest Benchmark")
	benchmarkGit(b, root, "commit", "--allow-empty", "-m", "init")
	benchmarkGit(b, root, "branch", "-M", "main")
	benchmarkGit(b, root, "update-ref", "refs/remotes/origin/main", "HEAD")

	b.ResetTimer()
	for b.Loop() {
		resolved, err := git.ResolveComparisonRef(context.Background(), root, "main")
		if err != nil || resolved.Ref != "origin/main" || resolved.OID == "" {
			b.Fatalf("resolved=%+v err=%v", resolved, err)
		}
	}
}

func benchmarkGit(b *testing.B, dir string, args ...string) {
	b.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		b.Fatalf("git %v failed: %v\n%s", args, err, output)
	}
}
