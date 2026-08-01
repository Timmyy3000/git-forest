package pathutil

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
)

func CheckCollision(candidate string, existing []string) error {
	cleanCandidate := filepath.Clean(candidate)
	candidateKey := collisionKey(cleanCandidate)
	for _, path := range existing {
		cleanExisting := filepath.Clean(path)
		existingKey := collisionKey(cleanExisting)
		if candidateKey == existingKey {
			return fmt.Errorf("path %s already exists", candidate)
		}
		if isPrefixPath(candidateKey, existingKey) {
			return fmt.Errorf("path %s conflicts with existing child %s", candidate, path)
		}
		if isPrefixPath(existingKey, candidateKey) {
			return fmt.Errorf("path %s conflicts with existing parent %s", candidate, path)
		}
	}
	return nil
}

func Contains(parent, child string) (bool, error) {
	parent, err := CanonicalPath(parent)
	if err != nil {
		return false, err
	}
	child, err = CanonicalPath(child)
	if err != nil {
		return false, err
	}
	if runtime.GOOS == "windows" {
		parent = strings.ToLower(parent)
		child = strings.ToLower(child)
	}
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false, err
	}
	return filepath.IsLocal(rel), nil
}

// CanonicalPath returns an absolute, symlink-resolved path for comparisons.
func CanonicalPath(path string) (string, error) {
	cleaned, err := NormalizePath(path)
	if err != nil {
		return "", err
	}
	evaluated, err := filepath.EvalSymlinks(cleaned)
	if err != nil {
		return "", err
	}
	return filepath.Clean(evaluated), nil
}

// NormalizePath returns an absolute, cleaned path with existing symlink
// prefixes resolved. Missing suffixes are preserved for comparisons of paths
// reported by Git with state paths that may not exist anymore.
func NormalizePath(path string) (string, error) {
	cleaned, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return "", err
	}
	cleaned, err = resolveExistingPrefix(cleaned)
	if err != nil {
		return "", err
	}
	if runtime.GOOS == "windows" {
		cleaned = strings.ToLower(cleaned)
	}
	return filepath.Clean(cleaned), nil
}

func resolveExistingPrefix(path string) (string, error) {
	current := path
	var suffix []string
	for {
		if evaluated, err := filepath.EvalSymlinks(current); err == nil {
			for i := len(suffix) - 1; i >= 0; i-- {
				evaluated = filepath.Join(evaluated, suffix[i])
			}
			return filepath.Clean(evaluated), nil
		}

		parent := filepath.Dir(current)
		if parent == current {
			return "", fmt.Errorf("resolve path %q: no existing ancestor", path)
		}
		suffix = append(suffix, filepath.Base(current))
		current = parent
	}
}

func collisionKey(path string) string {
	path = filepath.Clean(path)
	if runtime.GOOS == "windows" {
		return strings.ToLower(path)
	}
	return path
}

func isPrefixPath(parent, child string) bool {
	if parent == child {
		return false
	}
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return rel != "." && rel != "" && !strings.HasPrefix(rel, "..")
}
