# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Security Notice

This project is for security research and defensive purposes only. Do not generate code that could be used for unauthorized access. All security research must be conducted ethically and within legal boundaries.

## Project Overview

HackBrowserData is a CLI security research tool for extracting and decrypting browser data across Windows, macOS, and Linux. It supports Chromium-based browsers, Firefox, and Safari.

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

# Dependencies (MUST use go@1.20 to avoid bumping go directive)
# export GOROOT=$(brew --prefix go@1.20)/libexec && export PATH=$GOROOT/bin:$PATH
go mod tidy
go mod verify
```

## Chrome ABE Payload (Windows only)

Chrome 127+ cookies (v20) decrypt via a C payload reflectively injected into `chrome.exe`. Sources in `internal/abe/native/`; design in [RFC-016](rfcs/016-chrome-abe-integration.md).

```bash
make payload         # build the DLL (needs zig: brew install zig)
make build-windows   # cross-compile hack-browser-data.exe with payload embedded
make gen-layout      # regenerate Go layout constants from bootstrap_layout.h
make payload-clean   # rm internal/abe/payload/*.bin
```

- Default `go build` links a stub that errors at runtime when ABE is needed — contributors not touching Windows ABE need no zig.
- `-tags abe_embed` embeds the payload via `//go:embed` (see `internal/abe/payload/embed_windows.go` / `stub_windows.go`).

## Code Conventions

- **Platform code**: use build tags (`_darwin.go`, `_windows.go`, `_linux.go`)
- **C payload**: all sources in `internal/abe/native/` are first-party (no vendored C). See RFC-016 §9.2 for why Stephen Fewer's reflective loader was rejected.
- **Error handling**: `fmt.Errorf("context: %w", err)` for wrapping, never `_ =` to ignore errors
- **Logging**: `log.Debugf` for record-level diagnostics, `log.Infof` for user-facing progress/status, `log.Warnf` for unexpected conditions. Extract methods should return errors, not log them.
- **Naming**: follow Go conventions and the settled names in `rfcs/004-naming-decisions.md` — `Def` not `BrowserConfig`, `Extract` not `BrowsingData`
- **Comment width**: wrap comments at 140 columns (matches `.golangci.yml` `lll.line-length`)
- **Tests**: `internal/browser/browsertest` builds real fixture files (SQLite, JSON, LevelDB, plist) into a `t.TempDir()` — never a live browser, never a database mock; goldens live in each package's `testdata/` and `go test ./... -update` regenerates them
- **Architecture**: see `rfcs/README.md` — design RFCs 000–012, format and technique RFCs 013–021

## Key Constraints

- `modernc.org/sqlite` pinned at v1.31.1 (v1.32+ requires Go 1.21)
- `tidwall/gjson` < v1.19, `golang.org/x/term` < v0.30, `golang.org/x/sys` at v0.30.0 — later releases raise their `go` directive above 1.20
- Code lives under `internal/`, never `pkg/`; `cmd/` is the only entry point
- No library API — `cmd/` is the only consumer of the packages below it
