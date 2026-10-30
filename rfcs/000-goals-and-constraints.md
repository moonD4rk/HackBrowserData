# RFC-000: Goals & Constraints

## Context

HackBrowserData extracts and decrypts local browser data (passwords, cookies, history, bookmarks, and related artifacts) from Chromium-based browsers, Firefox, and Safari across Windows, macOS, and Linux. It is MIT-licensed, widely used, and was presented at DEF CON. Its structure is a redesign of a codebase that worked but had grown in layers:

- a `Browser` interface whose constructor discovers profiles and whose `Extract` does acquisition, key retrieval, decryption, and result assembly in one call, re-implemented per browser;
- per-OS `platformBrowsers()` lists of `types.BrowserConfig`, joined by key to a separate Windows `winutil.Table`, plus `ChromiumYandex` / `ChromiumOpera` kinds switched on across files;
- discovery, validation, and output concerns tangled into the command handlers (`-c` and `-f` are checked only after discovery and the keychain prompt).

The redesign is of the _structure_. The decryption and acquisition techniques are published and sit behind stable package boundaries; the design is how the program is assembled around them.

## Goals

1. **Reference-grade architecture.** A newcomer should be able to read `internal/app` top to bottom and understand the whole run. The pipeline is explicit, not emergent.
2. **Operator experience.** Usage errors fail before any browser is touched or any keychain prompt appears. Every browser/data-type pair yields a reported outcome. Secrets are retrieved only when the requested output needs them. Output is deterministic.
3. **Compatibility.** Nobody's command line, script, or output parser has to change (see the contract below).
4. **Code quality that lasts.** Adding a browser, a data type, or an output format is a single, documented, test-covered motion. No hidden coupling between stages.
5. **Showcase readiness.** The tree, the names, and the data flow are good enough to put on a slide.

## Non-goals

- **No new capability.** No new browsers beyond those already supported, no new bypasses, no new decryption techniques. The work is organizational and qualitative.
- **No interface changes.** No renamed commands or flags, no new defaults, no new exit codes, no output schema changes. Additive, opt-in surface is allowed only where a DECIDED item calls for it ([RFC-009](009-cli-surface.md)).
- **No network features.** The tool stays fully offline. No telemetry, no update checks, no remote anything.
- **No GUI.** CLI only.
- **No public library API.** The module is consumed through its CLI. Everything below `cmd/` lives in `internal/` (see [RFC-002](002-package-layout.md)); the module path does not change and there is no v2.

## The compatibility contract (DECIDED)

The external interface is frozen at v1.1.0: every item below behaves exactly as it did there, and a compatibility test suite pins it ([RFC-011](011-testing-and-quality.md)).

| Surface           | Frozen as                                                                                                                             |
| ----------------- | ------------------------------------------------------------------------------------------------------------------------------------- |
| Commands          | `dump` (also the no-subcommand default, with its flags mirrored on the root), `dumpkeys`, `archive`, `restore`, `list`, `version`     |
| Flags             | every flag's name, shorthand, type, and default, on every command ([RFC-009](009-cli-surface.md))                                     |
| Default selection | `-b all`, `-c all`, `-f json`, `-d results`                                                                                           |
| Output files      | one `<category>.<ext>` per non-empty category (`password.csv`, `cookie.json`, …); `--zip` packs them into `<dir>/<basename(dir)>.zip` |
| Formats           | `csv` (UTF-8 BOM), `json` (two-space indent, v1.1.0's HTML escaping), `cookie-editor`                                                 |
| Fields            | snake_case keys and columns, `browser` and `profile` first, then v1.1.0's fields in v1.1.0's order                                    |
| Stamp values      | `browser` = the display name (`Chrome`, `Microsoft Edge`); `profile` = v1.1.0's profile name ([RFC-003](003-core-types.md))           |
| `keys.json`       | dump version `"2"`, v1.1.0's schema, readable by v1.1.0                                                                               |
| Archive           | zip layout `<browser-key>/<User Data layout>`, readable by v1.1.0 `restore`                                                           |
| Exit status       | `0` when the run completes (including partial results and "no browsers found"); `1` when it cannot start or finish                    |

Not part of the contract: log lines and the human-readable summary on stderr, and row order where v1.1.0's order is unspecified or wrong. Output differs from v1.1.0 only for listed bug fixes where v1.1.0's output was wrong — Firefox history order, Firefox bookmark fields, Safari bookmark IDs, Firefox logins from a profile whose key cannot be recovered, Yandex logins and cards whose data key cannot be recovered, the `browser` value `restore` stamps, and CocCoc cookies on a Windows machine that also has Yandex ([RFC-008](008-output-and-export.md)); each is called out where it is fixed.

## Hard constraints

| Constraint                  | Reason                                                                                                                                                                                                     |
| --------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Builds on **Go 1.20**       | Windows 7 support. No `log/slog`, `slices`, `maps`, `cmp`, or any stdlib addition from 1.21+.                                                                                                              |
| Dependency **pins**         | `modernc.org/sqlite` v1.31.1, `tidwall/gjson` < v1.19, `golang.org/x/term` < v0.30, `golang.org/x/sys` at v0.30.0 — later releases raise their `go` directive above 1.20 ([RFC-010](010-dependencies.md)). |
| **Compatible interface**    | The contract above.                                                                                                                                                                                        |
| **MIT** license             | Project licensing; third-party license hygiene per [RFC-010](010-dependencies.md).                                                                                                                         |
| **Windows / macOS / Linux** | Existing supported matrix; no platform regressions.                                                                                                                                                        |
| Fully **offline**           | Security tool run on evidence; it must not emit traffic.                                                                                                                                                   |

## Guiding principles

These are the invariants every later RFC is built on.

- **One explicit pipeline.** A run is a fixed sequence of stages (see [RFC-001](001-architecture-and-pipeline.md)). Stages do not call back into each other; data flows one way.
- **Snapshot-first.** Acquisition copies the browser's files into a snapshot; every parser reads only from the snapshot, never from the live profile, and acquisition is a stage with its own contract ([RFC-007](007-snapshot-and-acquisition.md)). A dump captures one user data dir at a time and removes its copies before the next, so copied secrets never outlive the extractions that read them.
- **Lazy secrets.** Keys are retrieved, and the macOS login keychain is opened, on the first decrypt that needs them, not at discovery time. A run that only touches non-secret data never prompts: `dump -b chrome -c history` on macOS opens no keychain.
- **Outcome per extraction.** Each extraction, one (user data dir, profile, data-type) unit, produces an outcome — `ok`, `partial`, `absent`, `unsupported`, or `error` — and the run reports all of them, together with any failure above an extraction, such as a user data dir that cannot be read. A single failure never aborts the rest.
- **Wire names follow Go names.** JSON keys and CSV columns are the snake_case form of the Go field names, enforced by lint. Because output is frozen, the record field names are frozen with it. (DECIDED)
- **`internal/` only.** No `pkg/`, no root-level library. With no exported Go API there is no Go-level compatibility to keep; the CLI contract above is the only compatibility obligation.

## What success looks like

- `go build ./cmd/hack-browser-data/` on a clean Go 1.20 toolchain, cross-compiling to all three OSes.
- The same snapshot produces the output files v1.1.0 produced, byte for byte, except for the listed bug fixes.
- Every command's flag set — names, shorthands, types, defaults — matches v1.1.0's, plus the additive `--keychain-pw-stdin`.
- A usage error (`-f bogus`) exits `1` before discovery and before any keychain prompt.
- A run with no decryptable targets still exits `0` and reports `absent`/`unsupported` honestly.
- Adding a plain Chromium fork touches the browser table and nothing structural.
- The dependency set is justified line-by-line in [RFC-010](010-dependencies.md).
