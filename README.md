<div align="center">

# Forest

**Repo-local Git worktrees for parallel agent work.**

Forest keeps worktrees visible inside the repository, tracks what each agent is doing, and helps close stale worktrees when branches are merged.

[Website](https://forest.timi.click) · [Agent Guide](https://forest.timi.click/agents.md) · [Contributing](CONTRIBUTING.md) · [Security](SECURITY.md) · [Changelog](CHANGELOG.md)

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
- Configures VS Code search and file watcher ignores so `.forest/worktrees` stays visible without being indexed
- Prints machine-readable output from every command with `--json`
- Automatically recovers stale same-host state locks before mutating state
- Keeps default reusable file copying small, and skips nested Claude worktrees when `.claude` is explicitly copied
- Adopts Git-known `.forest/worktrees/*` entries with `forest doctor --fix` if state was lost mid-add
- Repairs common state and setup issues with `forest doctor --fix`
- Includes embedded coding-agent instructions with a link to `https://forest.timi.click/agents.md`

## Install

Install the latest prebuilt binary:

```bash
curl -fsSL https://forest.timi.click/install.sh | sh
```

Install a pinned version:

```bash
curl -fsSL https://forest.timi.click/install.sh | FOREST_VERSION=v0.2.0 sh
```

The installer downloads the matching GitHub release asset for your OS and architecture, verifies `checksums.txt` when `sha256sum` or `shasum` is available, and installs `forest` into `/usr/local/bin` or `~/.local/bin`. Set `FOREST_INSTALL_DIR` to choose a different directory.

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

Forest keeps worktrees visible in VS Code, Cursor, and other VS Code forks, but `forest init` adds workspace settings that exclude `.forest/worktrees` from search and file watching:

```json
{
  "files.watcherExclude": {
    "**/.forest/worktrees/**": true
  },
  "search.exclude": {
    "**/.forest/worktrees/**": true
  }
}
```

If `.vscode/settings.json` uses JSONC comments, Forest refuses to rewrite it and prints a warning instead of stripping comments. Use `forest doctor --fix` after converting that file to plain JSON or adding the settings manually.

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
| `forest list` | Show active worktrees, agent activity, and live local integration |
| `forest status [name]` | Show detailed Git health; use `forest status <name> --diff` for one worktree's patch |
| `forest list --fast` / `forest status --fast` | Metadata-only output, skipping every Git check |
| `forest mark` | Update phase, agent, note, and last-seen activity |
| `forest path` | Print a managed worktree path |
| `forest close` | Remove a selected worktree |
| `forest close --merged --yes` | Remove safely integrated worktrees |
| `forest doctor` | Diagnose Forest state |
| `forest doctor --fix` | Repair fixable state and stale-lock issues |
| `forest agents` | Print embedded Forest instructions for coding agents |

## Copying Reusable Setup Files

Forest creates `.forest/config.toml` with reusable paths that should be copied into each worktree:

```toml
[add]
copy = [".env", ".env.local"]
```

This is intended for small local environment files. Larger agent state directories such as `.claude`, `.cursor`, `.agent`, or `skills` should be opt-in per repository after checking that they do not contain nested checkouts or large generated content. Forest always skips `.claude/worktrees/**` when `.claude` is explicitly copied. `forest doctor --fix` migrates only the exact old generated copy list to the safer default and leaves custom copy lists alone.

## Agent Workflow

Coding agents can discover the Forest workflow with:

```bash
forest agents
```

Agents should run this at the start of every Forest session so they use the instructions that shipped with the installed CLI. This works without web access. To print only the canonical online guide URL:

```bash
forest agents --url
```

The canonical guide is published at [forest.timi.click/agents.md](https://forest.timi.click/agents.md) — point agents there (or paste it into an `AGENTS.md`/`CLAUDE.md`) when they cannot run the CLI.

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

Release artifacts are published when a `v*` tag is pushed:

```bash
git tag v0.2.0
git push origin v0.2.0
```

The release workflow builds Linux, macOS, and Windows binaries for `amd64` and `arm64`, uploads archives, and publishes `checksums.txt`.

Formatting:

```bash
gofmt -w $(git ls-files '*.go')
```

## Status

Early-stage and actively evolving. Forest is useful now, but command details and state schema may change before a stable release.

## License

MIT - see [LICENSE](LICENSE).
