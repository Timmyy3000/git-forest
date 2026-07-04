# Changelog

All notable changes to Forest will be documented here.

## Unreleased

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
