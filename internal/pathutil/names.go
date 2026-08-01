package pathutil

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
)

type Mapping struct {
	Identity string
	Branch   string
	RelPath  string
}

func FromName(worktreeRoot, name string) (Mapping, error) {
	if name == "" {
		return Mapping{}, fmt.Errorf("name is required")
	}
	return mapIdentity(worktreeRoot, name, "forest/"+name)
}

func FromBranch(worktreeRoot, branch string) (Mapping, error) {
	if branch == "" {
		return Mapping{}, fmt.Errorf("branch is required")
	}
	return mapIdentity(worktreeRoot, branch, branch)
}

func mapIdentity(worktreeRoot, identity, branch string) (Mapping, error) {
	if err := ValidateIdentity(identity); err != nil {
		return Mapping{}, err
	}
	rel := filepath.Join(worktreeRoot, filepath.FromSlash(identity))
	return Mapping{Identity: identity, Branch: branch, RelPath: rel}, nil
}

func ValidateIdentity(identity string) error {
	if identity == "" {
		return fmt.Errorf("identity is required")
	}
	if strings.HasPrefix(identity, "/") || strings.HasPrefix(identity, "\\") {
		return fmt.Errorf("identity %q escapes the worktree root", identity)
	}
	if strings.Contains(identity, "//") || strings.Contains(identity, "\\\\") {
		return fmt.Errorf("identity %q contains an empty path segment", identity)
	}
	normalized := filepath.Clean(filepath.FromSlash(identity))
	if normalized == "." || normalized == string(filepath.Separator) {
		return fmt.Errorf("invalid identity %q", identity)
	}
	if filepath.IsAbs(identity) || strings.HasPrefix(normalized, ".."+string(filepath.Separator)) || normalized == ".." {
		return fmt.Errorf("identity %q escapes the worktree root", identity)
	}
	parts := strings.FieldsFunc(identity, func(r rune) bool {
		return r == '/' || r == '\\'
	})
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return fmt.Errorf("invalid identity segment %q in %q", part, identity)
		}
	}
	return nil
}

func EqualFoldOnCaseInsensitiveFS(a, b string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}
