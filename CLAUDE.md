# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Security Notice

This project is for security research and defensive purposes only. Do not generate code that could be used for unauthorized access. All security research must be conducted ethically and within legal boundaries.

## Project Overview

HackBrowserData is a CLI security research tool for extracting and decrypting browser data across Windows, macOS, and Linux. It supports Chromium-based browsers and Firefox.

**Constraint**: Must build with Go 1.20 (Windows 7 support). Do not use features from Go 1.21+ (e.g., `log/slog`, `slices`, `maps`, `cmp` packages).

## Development Commands

```bash
# Build (use go@1.20 for module operations)
go build ./cmd/hack-browser-data/

# Cross-compile
GOOS=windows GOARCH=amd64 go build ./cmd/hack-browser-data/
GOOS=linux GOARCH=amd64 go build ./cmd/hack-browser-data/

# Test
go test ./...
go test -v ./... -covermode=count -coverprofile=coverage.out

# Lint (requires golangci-lint v2)
golangci-lint run

# Format (gofumpt is stricter than gofmt)
gofumpt -l -w .
goimports -w -local github.com/moond4rk/hackbrowserdata .

# Spelling
typos

# Dependencies (MUST use go@1.20 to avoid bumping go directive)
# export GOROOT=$(brew --prefix go@1.20)/libexec && export PATH=$GOROOT/bin:$PATH
go mod tidy
go mod verify
```

## Chrome ABE Payload (Windows only)

Chrome 127+ cookies (v20) decrypt via a C payload reflectively injected into `chrome.exe`. Sources in `crypto/windows/abe_native/`; design in [RFC-010](rfcs/010-chrome-abe-integration.md).

```bash
make payload         # build the DLL (needs zig: brew install zig)
make build-windows   # cross-compile hack-browser-data.exe with payload embedded
make gen-layout      # regenerate Go layout constants from bootstrap_layout.h
make payload-clean   # rm crypto/*.bin
```

- Default `go build` links a stub that errors at runtime when ABE is needed — contributors not touching Windows ABE need no zig.
- `-tags abe_embed` embeds the payload via `//go:embed` (see `crypto/abe_embed_windows.go` / `crypto/abe_stub_windows.go`).

## Code Conventions

- **Platform code**: use build tags (`_darwin.go`, `_windows.go`, `_linux.go`)
- **C payload**: all sources in `crypto/windows/abe_native/` are first-party (no vendored C). See RFC-010 §9.2 for why Stephen Fewer's reflective loader was rejected.
- **Error handling**: `fmt.Errorf("context: %w", err)` for wrapping, never `_ =` to ignore errors
- **Logging**: `log.Debugf` for record-level diagnostics, `log.Infof` for user-facing progress/status, `log.Warnf` for unexpected conditions. Extract methods should return errors, not log them.
- **Naming**: follow Go conventions — `Config` not `BrowserConfig`, `Extract` not `BrowsingData`
- **Comment width**: wrap comments at 120 columns (matches `.golangci.yml` `lll.line-length`)
- **Tests**: use `t.TempDir()` for filesystem tests, `go-sqlmock` for database tests
- **Architecture**: see `rfcs/` for design documents

## Key Constraints

- `modernc.org/sqlite` pinned at v1.31.1 (v1.32+ requires Go 1.21)
- `golang.org/x/text` will be removed in refactoring (use 3-byte UTF-8 BOM instead)
- No `pkg/` + `internal/` directory structure — keep it simple
- No root-level library API — CLI calls `browser.PickBrowsers()` directly
