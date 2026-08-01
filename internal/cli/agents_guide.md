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

Normal `forest list` refreshes the local integration result (`merged`, `patch-equivalent`, or `unmerged`) for every managed worktree without fetching. It intentionally skips dirty state, ahead/behind counts, diffs, and next actions; those entries carry `"detailsSkipped": true` in JSON.

Use `forest list --recursive` (or `forest list -r`) from a parent directory to list the Forest repository at that directory and its immediate child Forest repositories. Recursive output labels the starting repository as `.` and immediate children as `./repo`. Repositories nested two or more levels deep are not scanned; run the command from their direct parent instead. Recursive JSON returns a `repositories` array, with worktrees and any load error scoped to each repository.

Use `forest status` for detailed Git health. It checks clean/dirty state, ahead/behind counts, integration, and diagnostics. If one or more checks cannot complete, JSON marks the entry with `"checksIncomplete": true` and the human Git state is `unknown`; do not trust the zero values in that state. Use `forest status <name> --diff` to print the staged and unstaged patch for one named worktree. Untracked files are reported as dirty but have no Git patch to print.

Add `--fast` to `list`/`status` when you only need Forest metadata: it skips every per-worktree Git check and marks each entry with `"checksSkipped": true`.

### 6. Cleanup Only When Told

```bash
forest close <name> --yes --json
forest close --merged --yes --json
```

Close only when the human instructs you to, or when the work is proven integrated. Dirty or unmerged worktrees are refused unless you pass `--include-dirty` or `--include-unmerged`; do not pass those without explicit human instruction.

If Forest cannot inspect a worktree's integration state, `forest close` refuses to remove it even with the include flags. Run `forest doctor --fix` to reconcile a manually removed or otherwise invalid worktree record before trying again.

Never delete `.forest/worktrees/*` directories or run `git worktree remove` manually. Always go through `forest close`.

## Repairing Stale Worktrees

When a worktree is listed in Forest state but its folder, `.git` marker, or Git registration is missing, inspect all three sources of truth before taking action:

```bash
forest status --json
forest doctor --json
git worktree list --porcelain
```

`forest doctor` reports healthy worktrees, stale residual folders, stale state records, prunable Git metadata, and valid Forest worktrees that are missing from Forest state. Use `forest doctor --fix --json` when repair is authorized. It removes stale records, cleans verified residual directories only inside `.forest/worktrees`, prunes obsolete Forest-scoped Git metadata, and adopts valid untracked Forest worktrees. The operation is safe to rerun.

Invalid worktrees are reported as incomplete by `forest status`; do not infer their dirty or integration state from the parent repository. Valid dirty worktrees remain untouched, and paths outside `.forest/worktrees` are preserved.

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
| `forest list` | Dashboard: name, agent, phase, live local integration | `--agent` `--phase` `--recursive` / `-r` `--fast` `--json` |
| `forest status [name]` | Detailed Git health; named worktree can print a diff | `--agent` `--phase` `--diff` `--fast` `--json` |
| `forest mark` | Update phase, agent, and note | `--phase` `--agent` `--note` `--json` |
| `forest path [name]` | Print a worktree path | `--current` `--json` |
| `forest close <name>` | Safely remove a finished worktree | `--merged` `--yes` `--include-dirty` `--include-unmerged` `--delete-branch` `--json` |
| `forest doctor` | Diagnose and repair setup/state issues | `--fix` `--json` |
| `forest agents` | Print this embedded guide | `--url` `--json` |
| `forest version` | Print version information | `--json` |
