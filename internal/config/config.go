package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	ForestDir   = ".forest"
	WorktreeDir = ".forest/worktrees"
	StateDir    = ".forest/state"
	ConfigPath  = ".forest/config.toml"
)

type Config struct {
	Copy []string
}

var legacyDefaultCopy = []string{".env", ".env.local", ".claude", ".cursor", ".agent", "skills"}

func Default() Config {
	return Config{Copy: []string{".env", ".env.local"}}
}

func Ensure(root string) error {
	for _, dir := range []string{filepath.Join(root, ForestDir), filepath.Join(root, WorktreeDir), filepath.Join(root, StateDir)} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	if err := ensureConfig(root); err != nil {
		return err
	}
	return EnsureGitignore(root)
}

func ensureConfig(root string) error {
	path := filepath.Join(root, ConfigPath)
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	return os.WriteFile(path, []byte("[add]\ncopy = [\".env\", \".env.local\"]\n"), 0o644)
}

func Load(root string) (Config, error) {
	path := filepath.Join(root, ConfigPath)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return Default(), nil
	}
	if err != nil {
		return Config{}, err
	}
	cfg := Default()
	copyValues := parseCopyList(string(data))
	if copyValues != nil {
		cfg.Copy = copyValues
	}
	return cfg, nil
}

func RepairLegacyCopyDefault(root string) (bool, error) {
	path := filepath.Join(root, ConfigPath)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	copyValues := parseCopyList(string(data))
	if !equalStrings(copyValues, legacyDefaultCopy) {
		return false, nil
	}
	return true, os.WriteFile(path, []byte("[add]\ncopy = [\".env\", \".env.local\"]\n"), 0o644)
}

func parseCopyList(data string) []string {
	section := ""
	lines := strings.Split(data, "\n")
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(stripTOMLComment(lines[i]))
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(strings.Trim(line, "[]"))
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok || strings.TrimSpace(key) != "copy" {
			continue
		}
		if section != "" && section != "add" {
			continue
		}
		for !strings.Contains(value, "]") && i+1 < len(lines) {
			i++
			value += "\n" + stripTOMLComment(lines[i])
		}
		return parseStringList(value)
	}
	return nil
}

func parseStringList(value string) []string {
	start := strings.Index(value, "[")
	end := strings.LastIndex(value, "]")
	if start == -1 || end == -1 || end <= start {
		return nil
	}
	body := value[start+1 : end]
	var values []string
	for _, raw := range strings.Split(body, ",") {
		value := strings.Trim(strings.TrimSpace(raw), "\"'")
		if value != "" {
			values = append(values, value)
		}
	}
	return values
}

func stripTOMLComment(line string) string {
	inSingle := false
	inDouble := false
	escaped := false
	for idx, r := range line {
		switch {
		case escaped:
			escaped = false
		case r == '\\' && inDouble:
			escaped = true
		case r == '\'' && !inDouble:
			inSingle = !inSingle
		case r == '"' && !inSingle:
			inDouble = !inDouble
		case r == '#' && !inSingle && !inDouble:
			return line[:idx]
		}
	}
	return line
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func EnsureGitignore(root string) error {
	path := filepath.Join(root, ".gitignore")
	var lines []string
	if file, err := os.Open(path); err == nil {
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.TrimSpace(line) == ".forest/" {
				_ = file.Close()
				return nil
			}
			lines = append(lines, line)
		}
		if err := scanner.Err(); err != nil {
			_ = file.Close()
			return err
		}
		_ = file.Close()
	} else if !os.IsNotExist(err) {
		return err
	}
	if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) != "" {
		lines = append(lines, "")
	}
	lines = append(lines, ".forest/")
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
}

func CopyReusable(root, worktree string, cfg Config) (copied, warnings []string) {
	for _, entry := range cfg.Copy {
		if filepath.IsAbs(entry) || strings.Contains(filepath.Clean(entry), "..") {
			warnings = append(warnings, fmt.Sprintf("skipped unsafe copy entry %s", entry))
			continue
		}
		src := filepath.Join(root, entry)
		dst := filepath.Join(worktree, entry)
		if _, err := os.Stat(src); os.IsNotExist(err) {
			warnings = append(warnings, fmt.Sprintf("missing reusable path %s", entry))
			continue
		}
		if err := copyPath(entry, src, dst); err != nil {
			warnings = append(warnings, fmt.Sprintf("copy %s failed: %v", entry, err))
			continue
		}
		copied = append(copied, entry)
	}
	return copied, warnings
}
