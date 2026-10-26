# HackBrowserData RFCs

This directory is HackBrowserData's design record. The design RFCs (000–012) describe how the program is organized — a single explicit pipeline, a flat `internal/` package tree, outcome-per-extraction reporting, and names that read the way Go wants them to read — while the tool's external interface stays exactly as it is in v1.1.0. The format and technique RFCs (013–021) describe how each browser stores and protects its data, and how the tool reads it.

Every RFC states the current design. When a decision changes, the RFC is edited in place and git keeps the history; a superseded RFC is removed rather than kept beside its replacement.

The design RFCs describe _architecture_ — how the program is organized, how data moves through it, what the packages and types are called, and which dependencies are chosen and why. They deliberately do **not** restate decryption algorithms, key-extraction techniques, or platform specifics. Those live in the format and technique RFCs and are referenced by boundary, not reproduced.

## Why rewrite

HackBrowserData is an MIT-licensed security-research tool with a large user base, presented at DEF CON. The rewrite serves three goals, in priority order:

1. **A clean architecture that reads as a reference implementation.** The code is read on stage and by every contributor; it should teach the design, not apologize for it.
2. **A better operator experience without breaking anyone.** Fast failures, honest reporting, no surprises — and every existing command line, script, and output consumer keeps working unchanged.
3. **Durable code quality.** One obvious way to add a browser, a data type, or an output format — verified by tests, not tribal knowledge.

## Hard constraints (non-negotiable)

- **Go 1.20.** Windows 7 support depends on it. No Go 1.21+ features (`log/slog`, `slices`, `maps`, `cmp`, generic stdlib helpers that landed later), and no dependency version whose `go` directive is above 1.20.
- **Compatible external interface.** Commands, flags, defaults, output files and their field names, the `keys.json` and archive formats, and exit codes stay as they are in v1.1.0 — the compatibility contract in [RFC-000](000-goals-and-constraints.md). The rewrite lands as a series of PRs on an integration branch and reaches `main` in one fast-forward, on the existing module path; there is no v2 ([RFC-012](012-migration-roadmap.md)).
- **MIT license**, cross-platform (Windows, macOS, Linux), fully offline. The tool never phones home.
- **No new offensive capability.** The rewrite reorganizes and hardens existing, published functionality. It does not add targets, bypasses, or techniques.

## Design

| RFC                                     | Title                   | What it settles                                                            |
| --------------------------------------- | ----------------------- | -------------------------------------------------------------------------- |
| [000](000-goals-and-constraints.md)     | Goals & Constraints     | Why we rewrite, the compatibility contract, guiding principles             |
| [001](001-architecture-and-pipeline.md) | Architecture & Pipeline | The seven stages, data flow, outcome model, exit status                    |
| [002](002-package-layout.md)            | Package Layout          | The `internal/` tree, dependency direction, why `internal/`                |
| [003](003-core-types.md)                | Core Types              | Every central type and its shape                                           |
| [004](004-naming-decisions.md)          | Naming Decisions        | Settled names and the alternatives considered                              |
| [005](005-browser-definitions.md)       | Browser Definitions     | The supported-browser table, per-fork overrides, how a fork is added       |
| [006](006-key-retrieval-and-crypto.md)  | Key Retrieval & Crypto  | `masterkey`, `crypto`, and `keychain` boundaries, key scopes, lazy secrets |
| [007](007-snapshot-and-acquisition.md)  | Snapshot & Acquisition  | Snapshot-first capture, user-data-dir identity, archive layout             |
| [008](008-output-and-export.md)         | Output & Export         | Formats, frozen wire names, deterministic ordering                         |
| [009](009-cli-surface.md)               | CLI Surface             | The unchanged command surface, validation order, keychain password input   |
| [010](010-dependencies.md)              | Dependencies            | Keep / drop / pin, with rationale                                          |
| [011](011-testing-and-quality.md)       | Testing & Quality       | Test strategy, compatibility tests, CI, lint                               |
| [012](012-migration-roadmap.md)         | Migration Roadmap       | How the rewrite is delivered, phase by phase                               |

## Formats and techniques

| RFC                                      | Title                                   | What it covers                                              |
| ---------------------------------------- | --------------------------------------- | ----------------------------------------------------------- |
| [013](013-chromium-data-storage.md)      | Chromium Data Storage                   | Where Chromium keeps each category, and the file formats    |
| [014](014-chromium-encryption.md)        | Chromium Encryption                     | Cipher versions per platform                                |
| [015](015-key-retrieval-mechanisms.md)   | Key Retrieval Mechanisms                | Where each OS keeps the master key, and the Safari keychain |
| [016](016-chrome-abe-integration.md)     | Chrome App-Bound Encryption Integration | The Windows `v20` key path and its payload                  |
| [017](017-yandex-decryption.md)          | Yandex Browser Decryption               | Yandex's own layer on top of Chromium's                     |
| [018](018-firefox-data-storage.md)       | Firefox Data Storage                    | Where Firefox keeps each category, and the file formats     |
| [019](019-firefox-encryption.md)         | Firefox Encryption                      | The NSS key and login encryption                            |
| [020](020-safari-data-storage.md)        | Safari Data Storage                     | Safari's files, profiles, and keychain passwords            |
| [021](021-windows-locked-file-bypass.md) | Windows Locked File Bypass              | Reading files a running browser keeps locked                |

## Status legend

Each design RFC marks decisions as one of:

- **DECIDED** — agreed; build to it.
- **DEFERRED** — intentionally left out of this rewrite; listed so it is not mistaken for an omission.

Naming decisions and the alternatives behind them live in [004](004-naming-decisions.md).
