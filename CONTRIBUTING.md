# Contributing to HackBrowserData

We appreciate your interest in contributing to HackBrowserData! This document provides guidelines for contributors.

## Getting Started

- Always base your work from the `main` branch.
- Before creating a Pull Request (PR), make sure there is a corresponding issue for your contribution. If there isn't one already, please create one.

## Go Version Constraint

This project **must build with Go 1.20** to maintain Windows 7 support. This is enforced by CI.

- Do **not** use features from Go 1.21+ (e.g., `log/slog`, `slices`, `maps`, `cmp` packages)
- Do **not** bump the `go` directive in `go.mod` beyond `go 1.20`
- `modernc.org/sqlite` is pinned at v1.31.1 (v1.32+ requires Go 1.21)

## Development Commands

```bash
# Build
go build ./cmd/hack-browser-data/

# Test
go test ./...

# Lint (requires golangci-lint v2)
golangci-lint run

# Format
gofumpt -l -w .
goimports -w -local github.com/moond4rk/hackbrowserdata .

# Spelling check
typos
```

## Pull Requests

When creating a PR, please follow these guidelines:

- Link your PR to the corresponding issue.
- Provide context in the PR description to help reviewers understand the changes.
- Include 'before' and 'after' examples if applicable.
- Include steps for functional testing or replication.
- If you're adding a new feature, make sure to include unit tests.

### Commit Message Convention

Use [Conventional Commits](https://www.conventionalcommits.org/) format:

```
feat: add support for new browser
fix: resolve cookie decryption on Windows
chore: update dependencies
docs: improve RFC documentation
refactor: simplify profile discovery logic
test: add extraction tests for Firefox
```

## Code Style

- **Platform code**: use build tags (`_darwin.go`, `_windows.go`, `_linux.go`)
- **Error handling**: use `fmt.Errorf("context: %w", err)` for wrapping; do not ignore errors unless it is deliberate best-effort cleanup (e.g. `Close`/`Remove`)
- **Naming**: follow Go conventions
- **Tests**: use `t.TempDir()` for filesystem tests
- **Architecture**: see `rfcs/` for design documents

## Questions

If you have any questions or need further guidance, please feel free to ask in the issue or PR, or [reach out to the maintainers](mailto:me@moond4rk.com). We will reply to you as soon as possible.

Thank you for your contribution!
