# Security

Forest is a local CLI that operates on Git repositories, worktrees, and repo-local state under `.forest`.

## Supported Versions

Forest is pre-1.0. Security fixes are made on the default branch until release channels exist.

## Reporting A Vulnerability

Please do not publish exploitable details in a public issue.

Use GitHub private vulnerability reporting if it is enabled for the repository. If it is not enabled, open a minimal public issue asking for a private contact path without including exploit details.

Useful details:

- Forest version or commit
- operating system
- affected command
- reproduction steps
- expected and actual behavior

## Security Model

Forest assumes the local repository owner controls the repository and its `.forest` state. It still treats state paths defensively:

- worktree identities reject absolute paths and traversal
- managed paths must stay under `.forest/worktrees`
- current-worktree inference resolves symlinks before containment checks
- mutating state operations use a lock file
- `forest doctor --fix` only repairs issues it can classify safely

Forest does not run remote code or expose a network service.
