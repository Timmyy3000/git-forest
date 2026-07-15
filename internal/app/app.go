package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Timmyy3000/git-forest/internal/config"
	"github.com/Timmyy3000/git-forest/internal/git"
	"github.com/Timmyy3000/git-forest/internal/pathutil"
	"github.com/Timmyy3000/git-forest/internal/state"
)

type App struct{}

func New() *App { return &App{} }

type InitResult struct {
	ForestDir string   `json:"forestDir"`
	Warnings  []string `json:"warnings,omitempty"`
}

type AddOptions struct {
	Name   string
	Branch string
	From   string
	Agent  string
	Fetch  bool
	Quiet  bool
}

type AddResult struct {
	Name     string   `json:"name"`
	Branch   string   `json:"branch"`
	Path     string   `json:"path"`
	Copied   []string `json:"copied,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

type ListOptions struct {
	Name      string
	Agent     string
	Phase     string
	Detailed  bool
	Recursive bool
	// Fast skips the per-worktree git checks (dirty, ahead/behind,
	// integration) so output is instant; skipped views carry ChecksSkipped.
	Fast bool
}

type StatusOptions struct {
	Name  string
	Agent string
	Phase string
	Fast  bool
	Diff  bool
}

type WorktreeView struct {
	Name        string    `json:"name"`
	Branch      string    `json:"branch"`
	Path        string    `json:"path"`
	Agent       string    `json:"agent,omitempty"`
	Phase       string    `json:"phase,omitempty"`
	Note        string    `json:"note,omitempty"`
	Updated     time.Time `json:"updated"`
	Dirty       bool      `json:"dirty"`
	Ahead       int       `json:"ahead"`
	Behind      int       `json:"behind"`
	Integration string    `json:"integration"`
	// IntegrationError explains why Integration is unknown.
	IntegrationError string `json:"integrationError,omitempty"`
	// CheckError captures failures from detailed Git health checks.
	CheckError string `json:"checkError,omitempty"`
	// ChecksIncomplete marks detailed output with one or more unavailable Git
	// checks. Dirty, Ahead, and Behind must not be trusted in that state.
	ChecksIncomplete bool   `json:"checksIncomplete,omitempty"`
	Next             string `json:"next"`
	// ChecksSkipped marks that Dirty, Ahead, Behind, and Integration were
	// not computed (--fast); their zero values carry no meaning.
	ChecksSkipped bool `json:"checksSkipped,omitempty"`
	// DetailsSkipped marks an integration-only view. Dirty, Ahead, Behind,
	// and Next were not computed and their zero values carry no meaning.
	DetailsSkipped bool `json:"detailsSkipped,omitempty"`
}

type ListResult struct {
	Worktrees     []WorktreeView `json:"worktrees"`
	Diff          string         `json:"diff,omitempty"`
	DiffRequested bool           `json:"-"`
}

type RepositoryList struct {
	Path      string         `json:"path"`
	Worktrees []WorktreeView `json:"worktrees"`
	Error     string         `json:"error,omitempty"`
}

type RecursiveListResult struct {
	Repositories []RepositoryList `json:"repositories"`
	Warnings     []string         `json:"warnings,omitempty"`
}

type MarkOptions struct {
	Name  string
	Phase string
	Agent string
	Note  string
	// AgentSet/NoteSet distinguish an explicitly passed empty value
	// (clear the field) from an omitted flag (keep the stored value).
	AgentSet bool
	NoteSet  bool
}

type MarkResult struct {
	Name  string `json:"name"`
	Phase string `json:"phase"`
}

type CloseOptions struct {
	Name            string
	Merged          bool
	Yes             bool
	IncludeDirty    bool
	IncludeUnmerged bool
	DeleteBranch    bool
}

type CloseResult struct {
	Closed  []string  `json:"closed"`
	Skipped []Skipped `json:"skipped"`
}

type Skipped struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

type DoctorResult struct {
	Checks []Check `json:"checks"`
}
type Check struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

func (a *App) Init(ctx context.Context) (InitResult, error) {
	root, err := git.Root(ctx, ".")
	if err != nil {
		return InitResult{}, err
	}
	if err := config.Ensure(root); err != nil {
		return InitResult{}, err
	}
	store, err := state.Load(root)
	if err != nil {
		return InitResult{}, err
	}
	store.DefaultBase = git.DefaultBranch(ctx, root)
	if err := state.Save(root, store); err != nil {
		return InitResult{}, err
	}
	result := InitResult{ForestDir: filepath.Join(root, config.ForestDir)}
	if _, err := config.EnsureVSCodeIgnores(root); err != nil {
		result.Warnings = append(result.Warnings, "could not update VS Code ignores: "+err.Error())
	}
	return result, nil
}

func (a *App) Add(ctx context.Context, opts AddOptions) (AddResult, error) {
	root, err := git.Root(ctx, ".")
	if err != nil {
		return AddResult{}, err
	}
	if err := config.Ensure(root); err != nil {
		return AddResult{}, err
	}
	var result AddResult
	err = state.WithLock(root, "forest add", func() error {
		store, err := state.Load(root)
		if err != nil {
			return err
		}
		base := opts.From
		if base == "" {
			base = store.DefaultBase
		}
		if base == "" {
			base = git.DefaultBranch(ctx, root)
		}
		if err := git.ValidateRevision(base); err != nil {
			return err
		}
		mapping, err := mapAdd(opts)
		if err != nil {
			return err
		}
		if err := git.ValidateRevision(mapping.Branch); err != nil {
			return err
		}
		existing := make([]string, 0, len(store.Worktrees))
		for _, wt := range store.Worktrees {
			existing = append(existing, filepath.Join(root, wt.Path))
		}
		absPath := filepath.Join(root, mapping.RelPath)
		if err := pathutil.CheckCollision(absPath, existing); err != nil {
			return err
		}
		if opts.Fetch {
			if err := git.Fetch(ctx, root); err != nil {
				return err
			}
		}
		branchExists := git.BranchExists(ctx, root, mapping.Branch)
		cfg, err := config.Load(root)
		if err != nil {
			return err
		}
		if err := git.WorktreeAdd(ctx, root, absPath, mapping.Branch, base, branchExists); err != nil {
			return err
		}
		now := time.Now().UTC()
		actor := state.Actor{Kind: "human"}
		if opts.Agent != "" {
			actor = state.Actor{Kind: "agent", Name: opts.Agent}
		}
		worktree := state.Worktree{
			ID:        mapping.Identity,
			Name:      mapping.Identity,
			Branch:    mapping.Branch,
			Path:      mapping.RelPath,
			Base:      base,
			CreatedAt: now,
			CreatedBy: actor,
			Activity:  state.Activity{Phase: "claimed", Agent: opts.Agent, LastSeenAt: now},
			Status:    state.Status{LastKnown: "creating", LastCheckedAt: now},
		}
		store.Worktrees = append(store.Worktrees, worktree)
		if err := state.Save(root, store); err != nil {
			return err
		}
		_ = state.AppendEvent(root, state.Event{Time: now, Type: "creating", ID: mapping.Identity})
		copied, warnings := config.CopyReusable(root, absPath, cfg)
		worktree.Status = state.Status{LastKnown: "active", LastCheckedAt: time.Now().UTC()}
		if _, idx, ok := store.Find(mapping.Identity); ok {
			store.Worktrees[idx] = worktree
		}
		if err := state.Save(root, store); err != nil {
			return err
		}
		_ = state.AppendEvent(root, state.Event{Time: time.Now().UTC(), Type: "created", ID: mapping.Identity})
		result = AddResult{Name: mapping.Identity, Branch: mapping.Branch, Path: absPath, Copied: copied, Warnings: warnings}
		return nil
	})
	return result, err
}

func mapAdd(opts AddOptions) (pathutil.Mapping, error) {
	if opts.Branch != "" && opts.Name == "" {
		return pathutil.FromBranch(config.WorktreeDir, opts.Branch)
	}
	if opts.Name != "" && opts.Branch == "" {
		return pathutil.FromName(config.WorktreeDir, opts.Name)
	}
	if opts.Name != "" && opts.Branch != "" {
		mapping, err := pathutil.FromName(config.WorktreeDir, opts.Name)
		mapping.Branch = opts.Branch
		return mapping, err
	}
	return pathutil.Mapping{}, fmt.Errorf("name or branch is required")
}

func (a *App) List(ctx context.Context, opts ListOptions) (ListResult, error) {
	root, err := git.Root(ctx, ".")
	if err != nil {
		return ListResult{}, err
	}
	return a.listAt(ctx, root, opts)
}

func (a *App) ListRecursive(ctx context.Context, opts ListOptions) (RecursiveListResult, error) {
	start, err := os.Getwd()
	if err != nil {
		return RecursiveListResult{}, err
	}
	roots, warnings, err := discoverForestRoots(ctx, start)
	if err != nil {
		return RecursiveListResult{}, err
	}
	result := RecursiveListResult{Repositories: make([]RepositoryList, 0, len(roots)), Warnings: warnings}
	for _, root := range roots {
		if err := ctx.Err(); err != nil {
			return RecursiveListResult{}, err
		}
		repository := RepositoryList{Path: relativeRepositoryPath(start, root), Worktrees: []WorktreeView{}}
		listed, err := a.listAt(ctx, root, opts)
		if err != nil {
			repository.Error = err.Error()
		} else {
			repository.Worktrees = listed.Worktrees
		}
		result.Repositories = append(result.Repositories, repository)
	}
	return result, nil
}

func (a *App) listAt(ctx context.Context, root string, opts ListOptions) (ListResult, error) {
	store, err := state.Load(root)
	if err != nil {
		return ListResult{}, err
	}
	var selected []state.Worktree
	for _, wt := range store.Worktrees {
		if opts.Name != "" && wt.ID != opts.Name && wt.Name != opts.Name && wt.Branch != opts.Name {
			continue
		}
		if opts.Agent != "" && wt.Activity.Agent != opts.Agent {
			continue
		}
		if opts.Phase != "" && wt.Activity.Phase != opts.Phase {
			continue
		}
		selected = append(selected, wt)
	}
	if opts.Name != "" && len(selected) == 0 {
		return ListResult{}, fmt.Errorf("unknown worktree %s", opts.Name)
	}
	views := make([]WorktreeView, len(selected))
	var wg sync.WaitGroup
	for i, wt := range selected {
		view := WorktreeView{
			Name:    wt.Name,
			Branch:  wt.Branch,
			Path:    filepath.Join(root, wt.Path),
			Agent:   wt.Activity.Agent,
			Phase:   wt.Activity.Phase,
			Note:    wt.Activity.Note,
			Updated: wt.Activity.LastSeenAt,
		}
		if opts.Fast {
			view.ChecksSkipped = true
			view.Integration = "unknown"
			views[i] = view
			continue
		}
		if !opts.Detailed {
			wg.Add(1)
			go func(i int, view WorktreeView, base string) {
				defer wg.Done()
				view.Integration, view.IntegrationError = collectIntegration(ctx, view.Path, base)
				view.DetailsSkipped = true
				view.ChecksIncomplete = view.IntegrationError != ""
				views[i] = view
			}(i, view, wt.Base)
			continue
		}
		wg.Add(1)
		go func(i int, view WorktreeView, base, phase string) {
			defer wg.Done()
			checks := collectGitChecks(ctx, view.Path, base)
			view.Dirty = checks.dirty
			view.Ahead = checks.ahead
			view.Behind = checks.behind
			view.Integration = checks.integration
			view.IntegrationError = checks.integrationError
			view.CheckError = checks.checkError
			view.ChecksIncomplete = checks.incomplete
			if checks.checkError == "" {
				view.Next = nextAction(checks.dirty, checks.integration, phase)
			}
			views[i] = view
		}(i, view, wt.Base, wt.Activity.Phase)
	}
	wg.Wait()
	return ListResult{Worktrees: views}, nil
}

func discoverForestRoots(ctx context.Context, start string) ([]string, []string, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	entries, err := os.ReadDir(start)
	if err != nil {
		return nil, nil, err
	}
	candidates := make([]string, 0, len(entries)+1)
	candidates = append(candidates, start)
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		if entry.IsDir() {
			candidates = append(candidates, filepath.Join(start, entry.Name()))
		}
	}

	var roots []string
	var warnings []string
	for _, path := range candidates {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		forestDir := filepath.Join(path, config.ForestDir)
		info, err := os.Stat(forestDir)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("skipped %s: %v", relativeRepositoryPath(start, path), err))
			continue
		}
		if !info.IsDir() {
			warnings = append(warnings, fmt.Sprintf("skipped %s: %s is not a directory", relativeRepositoryPath(start, path), config.ForestDir))
			continue
		}
		root, err := git.Root(ctx, path)
		if err != nil || !samePath(root, path) {
			continue
		}
		roots = append(roots, path)
	}
	sort.Strings(roots)
	return roots, warnings, nil
}

func relativeRepositoryPath(start, root string) string {
	rel, err := filepath.Rel(start, root)
	if err != nil || rel == "." {
		return "."
	}
	return "./" + filepath.ToSlash(rel)
}

func samePath(left, right string) bool {
	var err error
	left, err = pathutil.CanonicalPath(left)
	if err != nil {
		return false
	}
	right, err = pathutil.CanonicalPath(right)
	if err != nil {
		return false
	}
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		return strings.EqualFold(left, right)
	}
	return left == right
}

func (a *App) Status(ctx context.Context, opts StatusOptions) (ListResult, error) {
	if opts.Diff && opts.Name == "" {
		return ListResult{}, fmt.Errorf("--diff requires a worktree name")
	}
	if opts.Diff && opts.Fast {
		return ListResult{}, fmt.Errorf("--diff cannot be used with --fast")
	}
	result, err := a.List(ctx, ListOptions{
		Name:     opts.Name,
		Agent:    opts.Agent,
		Phase:    opts.Phase,
		Detailed: !opts.Fast,
		Fast:     opts.Fast,
	})
	if err != nil {
		return ListResult{}, err
	}
	if !opts.Diff {
		return result, nil
	}
	if len(result.Worktrees) != 1 {
		return ListResult{}, fmt.Errorf("--diff requires exactly one managed worktree")
	}
	result.Diff, err = git.Diff(ctx, result.Worktrees[0].Path)
	if err != nil {
		return ListResult{}, err
	}
	result.DiffRequested = true
	return result, nil
}

// gitCheckSlots bounds concurrent git subprocesses across all worktree
// checks; spawning git is the dominant cost, especially on Windows.
var gitCheckSlots = make(chan struct{}, 8)

type gitChecks struct {
	dirty            bool
	ahead, behind    int
	integration      string
	integrationError string
	checkError       string
	incomplete       bool
}

// collectGitChecks runs the three per-worktree git checks concurrently.
// Each goroutine writes its own local before the WaitGroup barrier publishes
// the results; a cancelled context abandons queued checks instead of waiting
// on a semaphore slot.
func collectGitChecks(ctx context.Context, path, base string) gitChecks {
	var (
		wg           sync.WaitGroup
		dirty        bool
		ahead        int
		behind       int
		integration  string
		dirtyErr     error
		aheadErr     error
		integrateErr error
	)
	run := func(fn func(), cancelled func(error)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case gitCheckSlots <- struct{}{}:
			case <-ctx.Done():
				cancelled(ctx.Err())
				return
			}
			defer func() { <-gitCheckSlots }()
			fn()
		}()
	}
	run(func() { dirty, dirtyErr = git.Dirty(ctx, path) }, func(err error) { dirtyErr = err })
	run(func() { ahead, behind, aheadErr = git.AheadBehindWithError(ctx, path, base) }, func(err error) { aheadErr = err })
	run(func() { integration, integrateErr = git.Integration(ctx, path, base) }, func(err error) { integrateErr = err })
	wg.Wait()
	if dirtyErr != nil {
		dirty = true
	}
	if aheadErr != nil {
		ahead, behind = -1, -1
	}
	if integration == "" && integrateErr != nil {
		integration = "unknown"
	}
	var diagnostics []string
	if dirtyErr != nil {
		diagnostics = append(diagnostics, "dirty: "+dirtyErr.Error())
	}
	if aheadErr != nil {
		diagnostics = append(diagnostics, "ahead/behind: "+aheadErr.Error())
	}
	if integrateErr != nil {
		diagnostics = append(diagnostics, "integration: "+integrateErr.Error())
	}
	return gitChecks{
		dirty:            dirty,
		ahead:            ahead,
		behind:           behind,
		integration:      integration,
		integrationError: errorText(integrateErr),
		checkError:       strings.Join(diagnostics, "; "),
		incomplete:       len(diagnostics) > 0,
	}
}

func collectIntegration(ctx context.Context, path, base string) (string, string) {
	select {
	case gitCheckSlots <- struct{}{}:
	case <-ctx.Done():
		return "unknown", ctx.Err().Error()
	}
	defer func() { <-gitCheckSlots }()
	status, err := git.Integration(ctx, path, base)
	return status, errorText(err)
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func nextAction(dirty bool, integration, phase string) string {
	if phase == "blocked" {
		return "unblock agent"
	}
	if dirty {
		return "review changes"
	}
	if integration == "merged" || integration == "patch-equivalent" {
		return "close"
	}
	return "continue"
}

func (a *App) Mark(ctx context.Context, opts MarkOptions) (MarkResult, error) {
	root, err := git.Root(ctx, ".")
	if err != nil {
		return MarkResult{}, err
	}
	if opts.Name == "" {
		store, err := state.Load(root)
		if err != nil {
			return MarkResult{}, err
		}
		opts.Name, err = inferCurrent(store, root)
		if err != nil {
			return MarkResult{}, fmt.Errorf("cannot infer current worktree: %w", err)
		}
	}
	err = state.WithLock(root, "forest mark", func() error {
		store, err := state.Load(root)
		if err != nil {
			return err
		}
		wt, idx, ok := store.Find(opts.Name)
		if !ok {
			return fmt.Errorf("unknown worktree %s", opts.Name)
		}
		now := time.Now().UTC()
		wt.Activity.Phase = opts.Phase
		if opts.AgentSet {
			wt.Activity.Agent = opts.Agent
		}
		if opts.NoteSet {
			wt.Activity.Note = opts.Note
		}
		wt.Activity.LastSeenAt = now
		store.Worktrees[idx] = wt
		if err := state.Save(root, store); err != nil {
			return err
		}
		_ = state.AppendEvent(root, state.Event{Time: now, Type: "marked", ID: wt.ID, Detail: opts.Phase})
		return nil
	})
	return MarkResult{Name: opts.Name, Phase: opts.Phase}, err
}

func (a *App) Path(ctx context.Context, name string, current bool) (string, error) {
	root, err := git.Root(ctx, ".")
	if err != nil {
		return "", err
	}
	store, err := state.Load(root)
	if err != nil {
		return "", err
	}
	if current {
		if name, err = inferCurrent(store, root); err != nil {
			return "", err
		}
	}
	wt, _, ok := store.Find(name)
	if !ok {
		return "", fmt.Errorf("unknown worktree %s", name)
	}
	return filepath.Join(root, wt.Path), nil
}

func inferCurrent(store state.Store, root string) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	var firstResolveErr error
	var bestID string
	bestDepth := -1
	for _, wt := range store.Worktrees {
		if wt.Path == "" || wt.Path == "." || !filepath.IsLocal(wt.Path) {
			continue
		}
		abs := filepath.Join(root, wt.Path)
		contains, err := pathutil.Contains(abs, cwd)
		if err != nil {
			if firstResolveErr == nil {
				firstResolveErr = fmt.Errorf("%s: %w", wt.ID, err)
			}
			continue
		}
		if contains {
			depth := len(filepath.Clean(wt.Path))
			if depth > bestDepth {
				bestID = wt.ID
				bestDepth = depth
			}
		}
	}
	if bestID != "" {
		return bestID, nil
	}
	if firstResolveErr != nil {
		return "", fmt.Errorf("cannot resolve managed worktree paths: %w", firstResolveErr)
	}
	return "", fmt.Errorf("not inside a managed worktree")
}

func (a *App) Close(ctx context.Context, opts CloseOptions) (CloseResult, error) {
	root, err := git.Root(ctx, ".")
	if err != nil {
		return CloseResult{}, err
	}
	var result CloseResult
	err = state.WithLock(root, "forest close", func() error {
		store, err := state.Load(root)
		if err != nil {
			return err
		}
		var kept []state.Worktree
		matched := false
		for _, wt := range store.Worktrees {
			if !opts.Merged && wt.ID != opts.Name && wt.Name != opts.Name && wt.Branch != opts.Name {
				kept = append(kept, wt)
				continue
			}
			matched = true
			abs := filepath.Join(root, wt.Path)
			dirty, dirtyErr := git.Dirty(ctx, abs)
			if dirtyErr != nil {
				result.Skipped = append(result.Skipped, Skipped{Name: wt.Name, Reason: "worktree inaccessible: " + dirtyErr.Error()})
				kept = append(kept, wt)
				continue
			}
			integrated, integrationErr := git.Integration(ctx, abs, wt.Base)
			if dirty && !opts.IncludeDirty {
				result.Skipped = append(result.Skipped, Skipped{Name: wt.Name, Reason: "dirty"})
				kept = append(kept, wt)
				continue
			}
			if integrationErr != nil {
				result.Skipped = append(result.Skipped, Skipped{Name: wt.Name, Reason: "integration unknown: " + integrationErr.Error()})
				kept = append(kept, wt)
				continue
			}
			if integrated == "unmerged" && !opts.IncludeUnmerged {
				result.Skipped = append(result.Skipped, Skipped{Name: wt.Name, Reason: "unmerged"})
				kept = append(kept, wt)
				continue
			}
			if !opts.Yes {
				result.Skipped = append(result.Skipped, Skipped{Name: wt.Name, Reason: "requires --yes"})
				kept = append(kept, wt)
				continue
			}
			if err := git.WorktreeRemove(ctx, root, abs, dirty && opts.IncludeDirty); err != nil {
				result.Skipped = append(result.Skipped, Skipped{Name: wt.Name, Reason: err.Error()})
				kept = append(kept, wt)
				continue
			}
			if opts.DeleteBranch {
				_ = git.DeleteBranch(ctx, root, wt.Branch)
			}
			result.Closed = append(result.Closed, wt.Name)
			_ = state.AppendEvent(root, state.Event{Time: time.Now().UTC(), Type: "closed", ID: wt.ID})
		}
		if !opts.Merged && !matched {
			return fmt.Errorf("unknown worktree %s (run 'forest doctor' to check for git worktrees Forest is not tracking)", opts.Name)
		}
		store.Worktrees = kept
		return state.Save(root, store)
	})
	return result, err
}

func (a *App) Doctor(ctx context.Context, fix bool) (DoctorResult, error) {
	root, err := git.Root(ctx, ".")
	if err != nil {
		return DoctorResult{}, err
	}
	checks := []Check{{Name: "git repo", Status: "ok"}}
	if _, err := os.Stat(filepath.Join(root, config.ForestDir)); err == nil {
		checks = append(checks, Check{Name: ".forest", Status: "ok"})
	} else if fix {
		if err := config.Ensure(root); err != nil {
			return DoctorResult{}, err
		}
		checks = append(checks, Check{Name: ".forest", Status: "created"})
	} else {
		checks = append(checks, Check{Name: ".forest", Status: "missing"})
	}
	if fix {
		changed, err := config.EnsureVSCodeIgnores(root)
		if err != nil {
			checks = append(checks, Check{Name: "VS Code ignores", Status: "invalid: " + err.Error()})
		} else if changed {
			checks = append(checks, Check{Name: "VS Code ignores", Status: "updated"})
		} else {
			checks = append(checks, Check{Name: "VS Code ignores", Status: "ok"})
		}
	} else {
		ok, err := config.HasVSCodeIgnores(root)
		if err != nil {
			checks = append(checks, Check{Name: "VS Code ignores", Status: "invalid: " + err.Error()})
		} else if ok {
			checks = append(checks, Check{Name: "VS Code ignores", Status: "ok"})
		} else {
			checks = append(checks, Check{Name: "VS Code ignores", Status: "missing"})
		}
	}
	repairCopyDefaults := func() {
		changed, err := config.RepairLegacyCopyDefault(root)
		if err != nil {
			checks = append(checks, Check{Name: "copy defaults", Status: "invalid: " + err.Error()})
		} else if changed {
			checks = append(checks, Check{Name: "copy defaults", Status: "updated"})
		} else {
			checks = append(checks, Check{Name: "copy defaults", Status: "ok"})
		}
	}
	canMutate := true
	lockStatus, err := state.InspectLock(root)
	if err != nil {
		checks = append(checks, Check{Name: "state lock", Status: "invalid: " + err.Error()})
		canMutate = false
	} else if !lockStatus.Exists {
		checks = append(checks, Check{Name: "state lock", Status: "ok"})
	} else if lockStatus.Stale {
		checks = append(checks, Check{Name: "state lock", Status: "stale: " + lockStatus.Reason})
		if fix {
			latest, cleared, err := state.ClearStaleLock(root)
			if err != nil {
				return DoctorResult{}, err
			}
			switch {
			case cleared:
				checks = append(checks, Check{Name: "state lock cleanup", Status: "cleared"})
			case !latest.Exists:
				checks = append(checks, Check{Name: "state lock cleanup", Status: "already clear"})
			default:
				checks = append(checks, Check{Name: "state lock cleanup", Status: "skipped: " + latest.Reason})
				canMutate = false
			}
		} else {
			canMutate = false
		}
	} else {
		checks = append(checks, Check{Name: "state lock", Status: "active: " + lockStatus.Reason})
		canMutate = false
	}
	validateState := func() error {
		store, err := state.Load(root)
		if err != nil {
			checks = append(checks, Check{Name: "state file", Status: "invalid: " + err.Error()})
			return nil
		}
		checks = append(checks, Check{Name: "state file", Status: "ok"})
		adopted, untracked, reconcileErr := a.reconcileGitWorktrees(ctx, root, &store, fix && canMutate)
		if reconcileErr != nil {
			checks = append(checks, Check{Name: "git worktrees", Status: "invalid: " + reconcileErr.Error()})
			canMutate = false
		}
		for _, id := range untracked {
			checks = append(checks, Check{Name: "worktree " + id, Status: "untracked by Forest state"})
		}
		if adopted > 0 {
			checks = append(checks, Check{Name: "worktree adoption", Status: fmt.Sprintf("adopted %d git worktree(s)", adopted)})
		}
		var kept []state.Worktree
		removed := 0
		changed := adopted
		for _, wt := range store.Worktrees {
			keep := true
			if !validStatePath(wt.Path) {
				keep = false
				checks = append(checks, Check{Name: "state path " + wt.ID, Status: "invalid: " + wt.Path})
			} else if _, err := os.Stat(filepath.Join(root, wt.Path)); os.IsNotExist(err) {
				keep = false
				checks = append(checks, Check{Name: "worktree " + wt.ID, Status: "missing: " + wt.Path})
			} else if err != nil {
				checks = append(checks, Check{Name: "worktree " + wt.ID, Status: "invalid: " + err.Error()})
			} else if wt.Status.LastKnown == "creating" {
				checks = append(checks, Check{Name: "worktree " + wt.ID, Status: "creating"})
				if fix && canMutate {
					wt.Status = state.Status{LastKnown: "active", LastCheckedAt: time.Now().UTC()}
					checks = append(checks, Check{Name: "worktree " + wt.ID + " status cleanup", Status: "marked active"})
					changed++
				}
			}
			if keep {
				kept = append(kept, wt)
			} else {
				removed++
			}
		}
		if fix && canMutate && removed > 0 {
			store.Worktrees = kept
			if err := state.Save(root, store); err != nil {
				return err
			}
			checks = append(checks, Check{Name: "state path cleanup", Status: fmt.Sprintf("removed %d invalid record(s)", removed)})
		} else if removed == 0 {
			if fix && canMutate && changed > 0 {
				store.Worktrees = kept
				if err := state.Save(root, store); err != nil {
					return err
				}
			}
			checks = append(checks, Check{Name: "state paths", Status: "ok"})
		}
		return nil
	}
	if fix && canMutate {
		// Config repair mutates .forest/config.toml, so it runs under the
		// state lock alongside the other fix-mode mutations.
		err := state.WithLock(root, "forest doctor --fix", func() error {
			repairCopyDefaults()
			return validateState()
		})
		if err != nil {
			return DoctorResult{}, err
		}
	} else {
		if fix {
			checks = append(checks, Check{Name: "copy defaults", Status: "skipped: state is locked"})
		}
		if err := validateState(); err != nil {
			return DoctorResult{}, err
		}
	}
	return DoctorResult{Checks: checks}, nil
}

func (a *App) reconcileGitWorktrees(ctx context.Context, root string, store *state.Store, adopt bool) (int, []string, error) {
	worktrees, err := git.Worktrees(ctx, root)
	if err != nil {
		return 0, nil, err
	}
	now := time.Now().UTC()
	base := store.DefaultBase
	if base == "" {
		base = git.DefaultBranch(ctx, root)
	}
	var untracked []string
	adopted := 0
	for _, wt := range worktrees {
		rel, err := filepath.Rel(root, wt.Path)
		if err != nil {
			continue
		}
		rel = filepath.Clean(rel)
		if !validStatePath(rel) {
			continue
		}
		identityRel, err := filepath.Rel(filepath.Clean(config.WorktreeDir), rel)
		if err != nil || !filepath.IsLocal(identityRel) {
			continue
		}
		identity := filepath.ToSlash(identityRel)
		branch := wt.Branch
		if branch == "" {
			continue
		}
		if _, _, ok := store.Find(identity); ok {
			continue
		}
		if _, _, ok := store.Find(branch); ok {
			continue
		}
		untracked = append(untracked, identity)
		if !adopt {
			continue
		}
		store.Worktrees = append(store.Worktrees, state.Worktree{
			ID:        identity,
			Name:      identity,
			Branch:    branch,
			Path:      rel,
			Base:      base,
			CreatedAt: now,
			CreatedBy: state.Actor{Kind: "doctor"},
			Activity: state.Activity{
				Phase:      "adopted",
				Note:       "adopted by forest doctor --fix",
				LastSeenAt: now,
			},
			Status: state.Status{LastKnown: "active", LastCheckedAt: now},
		})
		_ = state.AppendEvent(root, state.Event{Time: now, Type: "adopted", ID: identity})
		adopted++
	}
	return adopted, untracked, nil
}

func validStatePath(path string) bool {
	if path == "" || path == "." || !filepath.IsLocal(path) {
		return false
	}
	clean := filepath.Clean(path)
	worktreeRoot := filepath.Clean(config.WorktreeDir)
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		clean = strings.ToLower(clean)
		worktreeRoot = strings.ToLower(worktreeRoot)
	}
	if clean == worktreeRoot {
		return false
	}
	rel, err := filepath.Rel(worktreeRoot, clean)
	return err == nil && filepath.IsLocal(rel)
}
