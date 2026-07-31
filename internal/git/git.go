package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
)

type WorktreeInfo struct {
	Path   string
	Branch string
	Head   string
}

func Run(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		// Git normally writes diagnostics to stderr, but some commands emit
		// useful details on stdout when stderr is empty.
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(stdout.String())
		}
		if msg == "" {
			msg = err.Error()
		}
		exitCode := -1
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		}
		return "", &RunError{Args: append([]string(nil), args...), Message: msg, ExitCode: exitCode, Cause: err}
	}
	return strings.TrimSpace(stdout.String()), nil
}

type RunError struct {
	Args     []string
	Message  string
	ExitCode int
	Cause    error
}

func (e *RunError) Error() string {
	return fmt.Sprintf("git %s: %s", strings.Join(e.Args, " "), e.Message)
}

func (e *RunError) Unwrap() error {
	return e.Cause
}

func Root(ctx context.Context, dir string) (string, error) {
	out, err := Run(ctx, dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	current := filepath.Clean(out)
	if hasForestDir(current) {
		return current, nil
	}
	if owner, ok := findForestOwner(ctx, current); ok {
		return owner, nil
	}
	if primary, ok := firstWorktree(ctx, current); ok {
		return primary, nil
	}
	return current, nil
}

func DefaultBranch(ctx context.Context, root string) string {
	if out, err := Run(ctx, root, "symbolic-ref", "--short", "refs/remotes/origin/HEAD"); err == nil && out != "" {
		if branch, ok := strings.CutPrefix(out, "origin/"); ok {
			return branch
		}
	}
	if out, err := Run(ctx, root, "branch", "--show-current"); err == nil && out != "" {
		return out
	}
	return "main"
}

func hasForestDir(root string) bool {
	_, err := os.Stat(filepath.Join(root, ".forest"))
	return err == nil
}

func findForestOwner(ctx context.Context, dir string) (string, bool) {
	worktrees, err := worktreePaths(ctx, dir)
	if err != nil {
		return "", false
	}
	for _, wt := range worktrees {
		if hasForestDir(wt) {
			return wt, true
		}
	}
	return "", false
}

func firstWorktree(ctx context.Context, dir string) (string, bool) {
	worktrees, err := worktreePaths(ctx, dir)
	if err != nil || len(worktrees) == 0 {
		return "", false
	}
	return worktrees[0], true
}

func worktreePaths(ctx context.Context, dir string) ([]string, error) {
	worktrees, err := Worktrees(ctx, dir)
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, wt := range worktrees {
		paths = append(paths, wt.Path)
	}
	return paths, nil
}

func Worktrees(ctx context.Context, dir string) ([]WorktreeInfo, error) {
	out, err := Run(ctx, dir, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	var worktrees []WorktreeInfo
	var current *WorktreeInfo
	flush := func() {
		if current != nil && current.Path != "" {
			worktrees = append(worktrees, *current)
		}
		current = nil
	}
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			flush()
			continue
		}
		if path, ok := strings.CutPrefix(line, "worktree "); ok {
			flush()
			current = &WorktreeInfo{Path: filepath.Clean(path)}
			continue
		}
		if current == nil {
			continue
		}
		if head, ok := strings.CutPrefix(line, "HEAD "); ok {
			current.Head = head
			continue
		}
		if branch, ok := strings.CutPrefix(line, "branch "); ok {
			current.Branch = strings.TrimPrefix(branch, "refs/heads/")
		}
	}
	flush()
	return worktrees, nil
}

func BranchExists(ctx context.Context, root, branch string) bool {
	_, err := Run(ctx, root, "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	return err == nil
}

func WorktreeAdd(ctx context.Context, root, path, branch, base string, branchExists bool) error {
	if branchExists {
		_, err := Run(ctx, root, "worktree", "add", path, branch)
		return err
	}
	_, err := Run(ctx, root, "worktree", "add", "-b", branch, path, base)
	return err
}

func WorktreeRemove(ctx context.Context, root, path string, force bool) error {
	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, path)
	_, err := Run(ctx, root, args...)
	return err
}

func DeleteBranch(ctx context.Context, root, branch string) error {
	_, err := Run(ctx, root, "branch", "-d", branch)
	return err
}

func Fetch(ctx context.Context, root string) error {
	_, err := Run(ctx, root, "fetch", "--all", "--prune")
	return err
}

type ComparisonRef struct {
	Ref    string
	Source string
	OID    string
}

// ResolveComparisonRef prefers the locally available origin tracking ref for
// an unqualified base branch. It never fetches; callers can expose the source
// so users know whether the comparison used remote-tracking or local history.
func ResolveComparisonRef(ctx context.Context, root, base string) (ComparisonRef, error) {
	if err := ValidateRevision(base); err != nil {
		return ComparisonRef{}, err
	}

	type candidate struct {
		ref    string
		source string
	}
	candidates := []candidate{{ref: base, source: "local"}}
	switch {
	case strings.HasPrefix(base, "origin/"):
		local := strings.TrimPrefix(base, "origin/")
		candidates = []candidate{
			{ref: base, source: "remote-tracking"},
			{ref: "refs/heads/" + local, source: "local"},
		}
	case strings.HasPrefix(base, "refs/remotes/"):
		local := strings.TrimPrefix(base, "refs/remotes/")
		local = strings.TrimPrefix(local, "origin/")
		candidates = []candidate{
			{ref: base, source: "remote-tracking"},
			{ref: "refs/heads/" + local, source: "local"},
		}
	case strings.HasPrefix(base, "refs/"):
		// Fully-qualified local refs should be resolved as requested rather
		// than being rewritten to an origin ref.
	default:
		candidates = []candidate{
			{ref: "origin/" + base, source: "remote-tracking"},
			{ref: "refs/heads/" + base, source: "local"},
			{ref: base, source: "local"},
		}
	}

	for _, candidate := range candidates {
		if oid, err := Run(ctx, root, "rev-parse", "--verify", candidate.ref+"^{commit}"); err == nil {
			return ComparisonRef{Ref: candidate.ref, Source: candidate.source, OID: oid}, nil
		}
	}

	refs := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		refs = append(refs, candidate.ref)
	}
	return ComparisonRef{}, fmt.Errorf("comparison base %q is unavailable (tried %s)", base, strings.Join(refs, ", "))
}

// ValidateRevision rejects values that Git could parse as command options.
func ValidateRevision(value string) error {
	if value == "" {
		return fmt.Errorf("revision is required")
	}
	if strings.HasPrefix(value, "-") {
		return fmt.Errorf("invalid revision %q: values beginning with '-' are not allowed", value)
	}
	if strings.IndexFunc(value, unicode.IsControl) >= 0 {
		return fmt.Errorf("invalid revision %q: control characters are not allowed", value)
	}
	if strings.IndexFunc(value, unicode.IsSpace) >= 0 {
		return fmt.Errorf("invalid revision %q: whitespace is not allowed", value)
	}
	return nil
}

func IsDirty(ctx context.Context, root string) bool {
	dirty, err := Dirty(ctx, root)
	return err != nil || dirty
}

func Dirty(ctx context.Context, root string) (bool, error) {
	out, err := Run(ctx, root, "status", "--porcelain=v1", "--untracked-files=all")
	if err != nil {
		return false, err
	}
	return out != "", nil
}

func AheadBehindWithError(ctx context.Context, root, base string) (int, int, error) {
	return AheadBehindRefWithError(ctx, root, base, "HEAD")
}

func AheadBehindRefWithError(ctx context.Context, root, base, head string) (int, int, error) {
	if err := ValidateRevision(base); err != nil {
		return 0, 0, err
	}
	if err := ValidateRevision(head); err != nil {
		return 0, 0, err
	}
	out, err := Run(ctx, root, "rev-list", "--left-right", "--count", base+"..."+head)
	if err != nil {
		return 0, 0, err
	}
	if out == "" {
		return 0, 0, fmt.Errorf("parse ahead/behind count: empty output")
	}
	fields := strings.Fields(out)
	if len(fields) != 2 {
		return 0, 0, fmt.Errorf("parse ahead/behind count %q", out)
	}
	ahead, err := strconv.Atoi(fields[1])
	if err != nil {
		return 0, 0, fmt.Errorf("parse ahead count %q: %w", fields[1], err)
	}
	behind, err := strconv.Atoi(fields[0])
	if err != nil {
		return 0, 0, fmt.Errorf("parse behind count %q: %w", fields[0], err)
	}
	return ahead, behind, nil
}

// Diff returns the combined staged and unstaged patch relative to HEAD.
// Untracked files remain visible through Dirty but have no Git patch to print.
func Diff(ctx context.Context, root string) (string, error) {
	return Run(ctx, root, "diff", "HEAD")
}

// Integration reports the current local relationship between HEAD and base.
// It keeps the fast per-commit patch check for simple histories, then uses a
// tree-level merge for merge-containing or otherwise unresolved histories.
func Integration(ctx context.Context, root, base string) (string, error) {
	return IntegrationRef(ctx, root, base, "HEAD")
}

// IntegrationWithComparison compares the worktree's checked-out HEAD against
// a resolved comparison ref while retaining the ref name for diagnostics.
func IntegrationWithComparison(ctx context.Context, root string, comparison ComparisonRef) (string, error) {
	return integrationRef(ctx, root, comparison.revision(), "HEAD", comparison.Ref)
}

// IntegrationRef reports the relationship between head and base using root as
// the repository directory. This lets callers inspect a managed branch from
// the primary worktree even when its checked-out worktree is unavailable.
// Aggregate comparisons use Git's --write-tree mode, which may materialize an
// unreachable tree object. Git reclaims those objects according to its normal
// unreachable-object expiry rules, while refs and checked-out worktree files
// remain untouched.
func IntegrationRef(ctx context.Context, root, base, head string) (string, error) {
	return integrationRef(ctx, root, base, head, base)
}

// IntegrationRefWithComparison compares a named head against a resolved
// comparison ref while retaining the ref name for diagnostics.
func IntegrationRefWithComparison(ctx context.Context, root string, comparison ComparisonRef, head string) (string, error) {
	return integrationRef(ctx, root, comparison.revision(), head, comparison.Ref)
}

func (comparison ComparisonRef) revision() string {
	if comparison.OID != "" {
		return comparison.OID
	}
	return comparison.Ref
}

func integrationRef(ctx context.Context, root, base, head, baseLabel string) (string, error) {
	if err := ValidateRevision(base); err != nil {
		return "unknown", err
	}
	if err := ValidateRevision(head); err != nil {
		return "unknown", err
	}
	info, err := os.Stat(root)
	if err != nil {
		return "unknown", fmt.Errorf("inspect repository: %w", err)
	}
	if !info.IsDir() {
		return "unknown", fmt.Errorf("inspect repository: not a directory")
	}
	if baseLabel == "" {
		baseLabel = base
	}
	headLabel := head
	resolvedBase, err := Run(ctx, root, "rev-parse", "--verify", base+"^{commit}")
	if err != nil {
		return "unknown", fmt.Errorf("resolve comparison base %s: %w", baseLabel, err)
	}
	resolvedHead, err := Run(ctx, root, "rev-parse", "--verify", head+"^{commit}")
	if err != nil {
		return "unknown", fmt.Errorf("resolve comparison head %s: %w", headLabel, err)
	}
	base = resolvedBase
	head = resolvedHead

	out, err := Run(ctx, root, "rev-list", "--right-only", "--cherry-pick", "--no-merges", "--count", base+"..."+head)
	if err != nil {
		return "unknown", err
	}
	count, err := strconv.Atoi(out)
	if err != nil {
		return "unknown", fmt.Errorf("parse unmerged commit count %q: %w", out, err)
	}
	if count < 0 {
		return "unknown", fmt.Errorf("parse unmerged commit count %q: negative value", out)
	}
	if count == 0 {
		mergeCount, err := rightOnlyMergeCount(ctx, root, base, head)
		if err != nil {
			return "unknown", err
		}
		if mergeCount == 0 {
			ancestor, err := IsAncestor(ctx, root, head, base)
			if err != nil {
				return "unknown", err
			}
			if ancestor {
				return "merged", nil
			}
			return "patch-equivalent", nil
		}
	}

	return aggregateIntegration(ctx, root, base, head, baseLabel, headLabel)
}

func rightOnlyMergeCount(ctx context.Context, root, base, head string) (int, error) {
	out, err := Run(ctx, root, "rev-list", "--right-only", "--merges", "--count", base+"..."+head)
	if err != nil {
		return 0, err
	}
	count, err := strconv.Atoi(out)
	if err != nil {
		return 0, fmt.Errorf("parse merge commit count %q: %w", out, err)
	}
	if count < 0 {
		return 0, fmt.Errorf("parse merge commit count %q: negative value", out)
	}
	return count, nil
}

func aggregateIntegration(ctx context.Context, root, base, head, baseLabel, headLabel string) (string, error) {
	stdout, err := runMergeTree(ctx, root, base, head)
	if err != nil {
		var runErr *RunError
		if errors.As(err, &runErr) && runErr.ExitCode == 1 {
			return "unmerged", nil
		}
		if strings.Contains(err.Error(), "CONFLICT") {
			return "unmerged", nil
		}
		return "unknown", err
	}

	fields := strings.Fields(stdout)
	if len(fields) == 0 {
		return "unknown", fmt.Errorf("git merge-tree %s %s returned no tree", baseLabel, headLabel)
	}
	baseTree, err := Run(ctx, root, "rev-parse", "--verify", base+"^{tree}")
	if err != nil {
		return "unknown", err
	}
	if fields[0] == baseTree {
		return "patch-equivalent", nil
	}
	return "unmerged", nil
}

func runMergeTree(ctx context.Context, root, base, head string) (string, error) {
	return Run(ctx, root, "merge-tree", "--write-tree", base, head)
}

func IsAncestor(ctx context.Context, root, ancestor, descendant string) (bool, error) {
	if err := ValidateRevision(ancestor); err != nil {
		return false, err
	}
	if err := ValidateRevision(descendant); err != nil {
		return false, err
	}
	cmd := exec.CommandContext(ctx, "git", "merge-base", "--is-ancestor", ancestor, descendant)
	cmd.Dir = root
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return false, ctx.Err()
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return false, nil
		}
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return false, fmt.Errorf("git merge-base --is-ancestor %s %s: %s", ancestor, descendant, msg)
	}
	return true, nil
}
