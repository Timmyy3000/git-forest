package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
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
