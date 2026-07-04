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
	parent, err := canonicalPath(parent)
	if err != nil {
		return false, err
	}
	child, err = canonicalPath(child)
	if err != nil {
		return false, err
	}
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		parent = strings.ToLower(parent)
		child = strings.ToLower(child)
	}
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false, err
	}
	return filepath.IsLocal(rel), nil
}

func canonicalPath(path string) (string, error) {
	cleaned, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return "", err
	}
	evaluated, err := filepath.EvalSymlinks(cleaned)
	if err == nil {
		return filepath.Clean(evaluated), nil
	}
	return cleaned, nil
}

func collisionKey(path string) string {
	path = filepath.Clean(path)
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
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
