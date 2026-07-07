# Changelog

All notable changes to Forest will be documented here.

## Unreleased

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
