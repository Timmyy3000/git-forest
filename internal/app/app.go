package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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
	Agent   string
	Phase   string
	Verbose bool
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
	Next        string    `json:"next"`
}

type ListResult struct {
	Worktrees []WorktreeView `json:"worktrees"`
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
		mapping, err := mapAdd(opts)
		if err != nil {
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
	store, err := state.Load(root)
	if err != nil {
		return ListResult{}, err
	}
	var views []WorktreeView
	for _, wt := range store.Worktrees {
		if opts.Agent != "" && wt.Activity.Agent != opts.Agent {
			continue
		}
		if opts.Phase != "" && wt.Activity.Phase != opts.Phase {
			continue
		}
		abs := filepath.Join(root, wt.Path)
		dirty := git.IsDirty(ctx, abs)
		ahead, behind := git.AheadBehind(ctx, abs, wt.Base)
		integration := git.Integrated(ctx, abs, wt.Base)
		view := WorktreeView{
			Name:        wt.Name,
			Branch:      wt.Branch,
			Path:        abs,
			Agent:       wt.Activity.Agent,
			Phase:       wt.Activity.Phase,
			Note:        wt.Activity.Note,
			Updated:     wt.Activity.LastSeenAt,
			Dirty:       dirty,
			Ahead:       ahead,
			Behind:      behind,
			Integration: integration,
			Next:        nextAction(dirty, integration, wt.Activity.Phase),
		}
		views = append(views, view)
	}
	return ListResult{Worktrees: views}, nil
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
		for _, wt := range store.Worktrees {
			if !opts.Merged && wt.ID != opts.Name && wt.Name != opts.Name && wt.Branch != opts.Name {
				kept = append(kept, wt)
				continue
			}
			abs := filepath.Join(root, wt.Path)
			dirty := git.IsDirty(ctx, abs)
			integrated := git.Integrated(ctx, abs, wt.Base)
			if dirty && !opts.IncludeDirty {
				result.Skipped = append(result.Skipped, Skipped{Name: wt.Name, Reason: "dirty"})
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
			if err := git.WorktreeRemove(ctx, root, abs); err != nil {
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
	if fix {
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
		adopted, untracked := a.reconcileGitWorktrees(ctx, root, &store, fix && canMutate)
		for _, id := range untracked {
			checks = append(checks, Check{Name: "worktree " + id, Status: "untracked by Forest state"})
		}
		if adopted > 0 {
			checks = append(checks, Check{Name: "worktree adoption", Status: fmt.Sprintf("adopted %d git worktree(s)", adopted)})
		}
		var kept []state.Worktree
		removed := 0
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
			if fix && canMutate && adopted > 0 {
				if err := state.Save(root, store); err != nil {
					return err
				}
			}
			checks = append(checks, Check{Name: "state paths", Status: "ok"})
		}
		return nil
	}
	if fix && canMutate {
		if err := state.WithLock(root, "forest doctor --fix", validateState); err != nil {
			return DoctorResult{}, err
		}
	} else if err := validateState(); err != nil {
		return DoctorResult{}, err
	}
	return DoctorResult{Checks: checks}, nil
}

func (a *App) reconcileGitWorktrees(ctx context.Context, root string, store *state.Store, adopt bool) (int, []string) {
	worktrees, err := git.Worktrees(ctx, root)
	if err != nil {
		return 0, nil
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
	return adopted, untracked
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
