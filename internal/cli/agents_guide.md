# Forest Agent Guide

Canonical guide: https://forest.timi.click/agents.md

Forest is a repo-local Git worktree manager for parallel coding-agent work.
Use it when the repository has Forest enabled and you need an isolated branch
and worktree for a task.

## Core Workflow

1. Inspect active work:

```bash
forest status
forest list
```

2. Create one worktree for your task:

```bash
forest add <short-task-name> --agent <your-name>
```

If the repo uses branch prefixes, use the branch as the identity:

```bash
forest add -b feat/login-copy --agent <your-name>
```

3. Move into the worktree and do all task work there:

```bash
cd "$(forest path <worktree-name>)"
```

4. Keep activity current:

```bash
forest mark --phase working --agent <your-name> --note "what you are doing"
forest mark --phase blocked --note "why you are blocked"
forest mark --phase ready --note "tests pass"
```

5. Close worktrees only when the human asks or the work is integrated:

```bash
forest close <name> --yes
forest close --merged --yes
```

## Rules For Agents

- One task, one Forest worktree.
- Do not work directly in the main checkout when Forest is in use.
- Always pass `--agent <your-name>` when creating a worktree.
- Use `forest mark` when you start, block, and finish.
- Do not delete `.forest/` contents or run `git worktree remove` manually.
- If state looks wrong, run `forest doctor` and report the output.

## Useful Commands

| Command | Purpose |
|---|---|
| `forest init` | Initialize Forest in the repo |
| `forest add <name>` | Create a repo-local worktree |
| `forest add -b <branch>` | Use branch name as worktree identity |
| `forest list` | Show active worktrees |
| `forest status` | Show grouped worktree status |
| `forest mark` | Update phase, agent, and note |
| `forest path <name>` | Print a worktree path |
| `forest close <name> --yes` | Safely remove a worktree |
| `forest doctor` | Diagnose Forest state |
| `forest agents` | Print this embedded guide |
| `forest agents --url` | Print only the canonical online guide URL |
