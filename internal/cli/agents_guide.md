# Forest - instructions for coding agents

Canonical guide: https://forest.timi.click/agents.md

You are reading this because you have been asked to use Forest while working in a repository. Follow this document exactly; it is the complete contract for agent work.

## Start Every Session

Run this first in every repository session where Forest may be in use:

```bash
forest agents
```

This command prints the embedded guide that shipped with the installed CLI, so it works even when web access is unavailable. Use the online guide only as the canonical published copy:

```bash
forest agents --url
```

## What Forest Is

Forest (`forest`) is a repo-local Git worktree manager built for parallel work by coding agents. Every task gets its own Git worktree inside the repository at `.forest/worktrees/<name>`. Forest tracks who is working on what in `.forest/state/worktrees.json`, and makes creation, status reporting, and cleanup safe.

Key facts:

- Worktrees live inside the repo, under `.forest/worktrees/`. The `.forest/` directory is gitignored.
- Git remains the source of truth; Forest state augments it.
- Commands work from anywhere inside the repo, including inside a managed worktree.
- Every command accepts `--json` for machine-readable output. Prefer it for automation.

## Workflow

### 1. Inspect Active Work

```bash
forest status --json
forest list --json
```

See what already exists. Never reuse another agent's dirty worktree.

### 2. Create Your Worktree

```bash
forest add <short-task-name> --agent <your-name> --json
```

This creates branch `forest/<short-task-name>`, a worktree at `.forest/worktrees/<short-task-name>`, and records you as the owner. The JSON result includes the worktree path; move into it and do all task work there.

If the repo has branch naming conventions, use `-b`. The branch name becomes the worktree identity:

```bash
forest add -b feat/login-copy --agent <your-name> --json
```

Use `--from <ref>` to base the branch on something other than the repo default branch:

```bash
forest add -b fix/pdf-signature --from dev --agent <your-name> --json
```

Forest copies repo-configured local files per `[add] copy` in `.forest/config.toml`. The default is intentionally small: `.env` and `.env.local`. Larger agent state directories should be opt-in per repository, and Forest always skips `.claude/worktrees/**`.

### 3. Work Inside The Worktree

```bash
cd "$(forest path <worktree-name>)"
```

Do not work directly in the main checkout when Forest is in use.

### 4. Report Activity

Run from inside your worktree, or name the worktree explicitly.

```bash
forest mark --phase working --agent <your-name> --note "what you are doing" --json
forest mark --phase blocked --note "why you are blocked" --json
forest mark --phase ready --note "tests pass" --json
```

Common phases: `claimed`, `working`, `waiting`, `blocked`, `ready`, `done`.

Mark at minimum when you start, when you cannot proceed, and when you finish. Omitting `--agent` or `--note` keeps the stored value; passing an empty string clears it.

### 5. Find Your Way Around

```bash
forest path <name>
forest path --current
forest list --json
forest status --json
```

### 6. Cleanup Only When Told

```bash
forest close <name> --yes --json
forest close --merged --yes --json
```

Close only when the human instructs you to, or when the work is proven integrated. Dirty or unmerged worktrees are refused unless you pass `--include-dirty` or `--include-unmerged`; do not pass those without explicit human instruction.

Never delete `.forest/worktrees/*` directories or run `git worktree remove` manually. Always go through `forest close`.

## Lock And Repair Rules

- If a command reports that Forest state is locked, wait briefly and retry once.
- Current Forest can automatically clear stale same-host locks before mutating state.
- If the lock persists, run `forest doctor --json` and report the findings.
- Do not manually delete lock files or `.forest/` contents.
- Run `forest doctor --fix` only when the human or repo instructions allow repair.

## Command Reference

| Command | Purpose | Key flags |
|---|---|---|
| `forest init` | Make repo Forest-managed | `--json` `--quiet` |
| `forest add <name>` | Create branch `forest/<name>` and worktree | `-b <branch>` `--from <ref>` `--agent` `--fetch` `--json` `--quiet` |
| `forest list` | Dashboard: name, agent, phase, git state, integration | `--agent` `--phase` `--verbose` `--json` |
| `forest status` | Grouped active / blocked / ready views | `--agent` `--phase` `--json` |
| `forest mark` | Update phase, agent, and note | `--phase` `--agent` `--note` `--json` |
| `forest path [name]` | Print a worktree path | `--current` `--json` |
| `forest close <name>` | Safely remove a finished worktree | `--merged` `--yes` `--include-dirty` `--include-unmerged` `--delete-branch` `--json` |
| `forest doctor` | Diagnose and repair setup/state issues | `--fix` `--json` |
| `forest agents` | Print this embedded guide | `--url` `--json` |
| `forest version` | Print version information | `--json` |
