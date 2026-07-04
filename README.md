<div align="center">

# Forest

**Repo-local Git worktrees for parallel agent work.**

Forest keeps worktrees visible inside the repository, tracks what each agent is doing, and helps close stale worktrees when branches are merged.

[Contributing](CONTRIBUTING.md) · [Security](SECURITY.md) · [Changelog](CHANGELOG.md)

</div>

---

## Why Forest Exists

Git worktrees are powerful, but the default workflow is easy to lose track of when humans and agents are working in parallel:

- worktrees often live outside the repo you are looking at
- it is hard to see which branches and agents are active
- cleanup is manual after PRs merge
- agents need repeatable setup files, env files, and repo standards copied into each workspace

Forest makes worktrees repo-local by default:

```text
repo/
  .forest/
    worktrees/
      feat/login-copy/
      chore/docs/
    state/
      worktrees.json
      events.jsonl
```

## What Forest Does Today

- Creates visible worktrees under `.forest/worktrees`
- Tracks worktree state under `.forest/state`
- Supports branch-first creation: `forest add -b feat/login-copy`
- Copies reusable setup files into new worktrees
- Shows dashboard-style `list` and `status` output
- Lets agents update activity with `forest mark`
- Infers the current managed worktree from any subdirectory
- Detects merged and patch-equivalent branches
- Removes merged or selected worktrees with `forest close`
- Repairs common state and lock issues with `forest doctor --fix`

## Install

From source:

```bash
go install github.com/Timmyy3000/git-forest/cmd/forest@latest
```

For local development:

```bash
git clone https://github.com/Timmyy3000/git-forest
cd git-forest
go build ./cmd/forest
```

## Quick Start

Initialize Forest in a Git repository:

```bash
forest init
```

Create a worktree with Forest's default branch naming:

```bash
forest add login-copy
```

This creates:

- worktree identity: `login-copy`
- branch: `forest/login-copy`
- path: `.forest/worktrees/login-copy`

Use your own branch convention:

```bash
forest add -b feat/login-copy
```

This uses `feat/login-copy` as both the branch and worktree identity.

See what is active:

```bash
forest list
forest status
```

Mark agent progress:

```bash
forest mark feat/login-copy --phase working --agent codex --note "updating copy"
forest mark --phase blocked --note "waiting on product decision"
```

Close worktrees after merge:

```bash
forest close --merged --yes
```

## Commands

| Command | Purpose |
|---|---|
| `forest init` | Create `.forest` directories, config, and ignore rules |
| `forest add [name]` | Create a repo-local worktree |
| `forest add -b <branch>` | Create or attach a worktree using the branch as the identity |
| `forest list` | Show active worktrees and agent activity |
| `forest status` | Show a dashboard summary |
| `forest mark` | Update phase, agent, note, and last-seen activity |
| `forest path` | Print a managed worktree path |
| `forest close` | Remove a selected worktree |
| `forest close --merged --yes` | Remove safely integrated worktrees |
| `forest doctor` | Diagnose Forest state |
| `forest doctor --fix` | Repair fixable state and stale-lock issues |

## Copying Reusable Setup Files

Forest creates `.forest/config.toml` with reusable paths that should be copied into each worktree:

```toml
[add]
copy = [".env", ".env.local", ".claude", ".cursor", ".agent", "skills"]
```

This is intended for agent workflows where every worktree needs the same repo standards, skills, or local environment files.

## Agent Workflow

A typical agent flow:

```bash
forest add -b feat/login-copy --agent codex
cd "$(forest path feat/login-copy)"
forest mark --phase working --note "implementing requested change"

# work, test, commit, open PR

forest mark --phase review --note "PR opened"
```

After the PR merges:

```bash
forest close --merged --yes --delete-branch
```

## Development

```bash
go test ./...
go vet ./...
go build ./cmd/forest
```

Formatting:

```bash
gofmt -w $(git ls-files '*.go')
```

## Status

Early-stage and actively evolving. Forest is useful now, but command details and state schema may change before a stable release.

## License

MIT - see [LICENSE](LICENSE).
