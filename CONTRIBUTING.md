# Contributing

Thanks for helping improve Forest.

## Development Setup

```bash
git clone https://github.com/Timmyy3000/git-forest
cd git-forest
go test ./...
go build ./cmd/forest
```

## Before Opening A PR

Run:

```bash
gofmt -w $(git ls-files '*.go')
go vet ./...
go test ./...
go build ./cmd/forest
```

Keep changes focused. For behavior changes, include tests when the code touches path safety, state handling, locks, or command semantics.

## Design Principles

- Keep worktrees visible and repo-local by default.
- Prefer explicit state and repairable workflows over hidden magic.
- Treat agent concurrency as a first-class use case.
- Avoid network operations unless the user explicitly opts in.
- Keep state repair conservative and explain skipped work.

## Pull Requests

PRs should include:

- what changed
- why it changed
- local validation performed
- any follow-up risks or known gaps

Forest uses Enkii and CI checks on pull requests. Address blocking review findings before merge.
