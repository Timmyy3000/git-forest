# Forest Review Instructions

Review this repository as a Go CLI for repo-local Git worktree management.

Prioritize actionable correctness findings over style comments. Focus on:

- Git worktree safety: avoid deleting dirty, unmerged, current, missing, or in-use worktrees without explicit user intent.
- Path safety: reject traversal, exact collisions, parent/child prefix collisions, symlink/junction surprises, and case-insensitive collisions on Windows/macOS.
- Root resolution: commands run from inside `.forest/worktrees/<name>/...` must resolve the owning repo's `.forest/`, not create nested Forest roots.
- State handling: `.forest/state/worktrees.json`, `.forest/state/events.jsonl`, and `.forest/state/lock` should remain consistent when commands fail or run concurrently.
- Agent dashboard behavior: `forest list`, `forest status`, and `forest mark` should preserve agent ownership, phase, notes, and last-seen activity unless the user explicitly changes them.
- Cross-platform behavior: Windows, macOS, and Linux should be considered for path separators, file locks, long paths, symlinks, and case sensitivity.
- Tests: risky path mapping, locking, state mutation, Git integration, and destructive command behavior should have meaningful test coverage.

Do not flag broad future-scope items unless the change makes them harder to implement later. The project is intentionally early-stage, so prefer small, concrete fixes with file and line references.

