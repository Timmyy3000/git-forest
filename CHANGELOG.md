# Changelog

All notable changes to Forest will be documented here.

## Unreleased

## v0.8.0 - 2026-08-01

- Hardened comparison and merge-tree checks by pinning remote base commits, selecting worktree heads deliberately, and surfacing lookup or cancellation failures.
- Added clearer diagnostics and warnings when closing unmerged worktrees or reusing incomplete comparison metadata.

## v0.7.0 - 2026-07-15

- Reconciles Forest state, physical worktree folders, `.git` markers, and Git registrations so stale residuals and prunable metadata are diagnosed explicitly.
- Makes `forest status` report invalid worktree paths as incomplete instead of inspecting the parent repository.
- Makes `forest doctor --fix` remove stale state, safely clean residual directories under `.forest/worktrees`, prune obsolete Forest Git metadata, and adopt valid untracked Forest worktrees.
- Makes `forest close` recognize stale residuals while preserving valid dirty worktrees and invalid paths that are not safe to remove.
- Adds cross-platform path normalization and regression coverage for Windows, macOS, nested branches, missing markers, prunable registrations, adoption, and containment safety.

## v0.6.0 - 2026-07-15

- Bound `forest list --recursive` discovery to the current directory and immediate children, avoiding expensive traversal of dependency, cache, build, and generated subtrees.

## v0.5.0 - 2026-07-10

- `forest list --recursive` (or `forest list -r`) discovers Forest-managed Git repositories below the current directory and groups their worktrees using relative repository paths.
- Recursive listing skips managed `.forest/worktrees`, preserves the normal integration-only list contract, supports `--fast`, and reports per-repository load errors without hiding healthy siblings.
- List output now adapts to terminal width while keeping tabular output for pipes and redirected output.

## v0.4.0 - 2026-07-10

- `forest list` now refreshes live local integration by default without running the full Git-health dashboard, substantially improving multi-worktree latency.
- `forest status [name]` provides detailed clean/dirty and ahead/behind health, while `forest status <name> --diff` prints the named worktree's staged and unstaged patch.
- Integration detection is patch-aware, and failed or cancelled Git inspection is surfaced explicitly instead of being reported as trusted status data.

## v0.3.0 - 2026-07-07

- `forest list` and `forest status` run their per-worktree git checks concurrently, cutting dashboard latency on repos with several worktrees.
- New `--fast` flag on `forest list` and `forest status` skips git checks entirely for instant state-only output, marked with `checksSkipped` in JSON.

## v0.2.0 - 2026-07-07

- Automatic stale same-host lock recovery before mutating state, with lock cleanup on SIGINT/SIGTERM/SIGHUP.
- Machine-readable `--json` output across the CLI, including structured lock error payloads with suggested actions.
- Safer reusable copy defaults, with `.claude/worktrees/**` excluded even when `.claude` is opted in and `doctor --fix` migration for the old generated copy list.
- `forest add` now records a creating state before copying reusable files, and `forest doctor --fix` can adopt Git-known Forest worktrees missing from state.
- `forest agents` command with embedded coding-agent instructions, `--url` for the online guide, and a first-step session contract for agents.

## v0.1.0 - 2026-07-04

- Initial Forest CLI foundation.
- Repo-local worktree creation under `.forest/worktrees`.
- State tracking under `.forest/state`.
- Agent activity updates with `forest mark`.
- Dashboard-style `list` and `status` output.
- Worktree cleanup with `forest close`.
- State and stale-lock repair with `forest doctor --fix`.
- VS Code search and file watcher ignores for `.forest/worktrees`.
- Enkii review workflow.
- GitHub release workflow for prebuilt Linux, macOS, and Windows binaries.
- Shell installer for downloading release assets without requiring Go.
- `forest version` command with release build metadata.
