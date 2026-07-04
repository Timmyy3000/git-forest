package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/oluwatimilehin/git-forest/internal/config"
	"github.com/oluwatimilehin/git-forest/internal/git"
	"github.com/oluwatimilehin/git-forest/internal/pathutil"
	"github.com/oluwatimilehin/git-forest/internal/state"
)

type App struct{}

func New() *App { return &App{} }

type InitResult struct{ ForestDir string }

type AddOptions struct {
	Name   string
	Branch string
	From   string
	Agent  string
	Fetch  bool
	Quiet  bool
}

type AddResult struct {
	Name     string
	Branch   string
	Path     string
	Copied   []string
	Warnings []string
}

type ListOptions struct {
	Agent   string
	Phase   string
	Verbose bool
}

type WorktreeView struct {
	Name        string
	Branch      string
	Path        string
	Agent       string
	Phase       string
	Note        string
	Updated     time.Time
	Dirty       bool
	Ahead       int
	Behind      int
	Integration string
	Next        string
}

type ListResult struct{ Worktrees []WorktreeView }

type MarkOptions struct {
	Name  string
	Phase string
	Agent string
	Note  string
}

type MarkResult struct {
	Name  string
	Phase string
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
	Closed  []string
	Skipped []Skipped
}

type Skipped struct {
	Name   string
	Reason string
}

type DoctorResult struct{ Checks []Check }
type Check struct {
	Name   string
	Status string
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
	return InitResult{ForestDir: filepath.Join(root, config.ForestDir)}, nil
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
		if err := git.WorktreeAdd(ctx, root, absPath, mapping.Branch, base, branchExists); err != nil {
			return err
		}
		cfg, _ := config.Load(root)
		copied, warnings := config.CopyReusable(root, absPath, cfg)
		now := time.Now().UTC()
		actor := state.Actor{Kind: "human"}
		if opts.Agent != "" {
			actor = state.Actor{Kind: "agent", Name: opts.Agent}
		}
		store.Worktrees = append(store.Worktrees, state.Worktree{
			ID:        mapping.Identity,
			Name:      mapping.Identity,
			Branch:    mapping.Branch,
			Path:      mapping.RelPath,
			Base:      base,
			CreatedAt: now,
			CreatedBy: actor,
			Activity:  state.Activity{Phase: "claimed", Agent: opts.Agent, LastSeenAt: now},
			Status:    state.Status{LastKnown: "active", LastCheckedAt: now},
		})
		if err := state.Save(root, store); err != nil {
			return err
		}
		_ = state.AppendEvent(root, state.Event{Time: now, Type: "created", ID: mapping.Identity})
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
	err = state.WithLock(root, "forest mark", func() error {
		store, err := state.Load(root)
		if err != nil {
			return err
		}
		if opts.Name == "" {
			opts.Name, _ = inferCurrent(store, root)
		}
		if opts.Name == "" {
			return fmt.Errorf("provide a worktree name when not inside a managed worktree")
		}
		wt, idx, ok := store.Find(opts.Name)
		if !ok {
			return fmt.Errorf("unknown worktree %s", opts.Name)
		}
		now := time.Now().UTC()
		wt.Activity.Phase = opts.Phase
		wt.Activity.Agent = opts.Agent
		wt.Activity.Note = opts.Note
		wt.Activity.LastSeenAt = now
		store.Worktrees[idx] = wt
		if err := state.Save(root, store); err != nil {
			return err
		}
		return state.AppendEvent(root, state.Event{Time: now, Type: "marked", ID: wt.ID, Detail: opts.Phase})
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
	cwd, _ := os.Getwd()
	cwd, _ = filepath.Abs(cwd)
	for _, wt := range store.Worktrees {
		abs := filepath.Join(root, wt.Path)
		rel, err := filepath.Rel(abs, cwd)
		if err == nil && (rel == "." || (rel != "" && !isParentTraversal(rel))) {
			return wt.ID, nil
		}
	}
	return "", fmt.Errorf("not inside a managed worktree")
}

func isParentTraversal(rel string) bool {
	return rel == ".." || len(rel) > 3 && rel[:3] == ".."+string(filepath.Separator)
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
	return DoctorResult{Checks: checks}, nil
}
