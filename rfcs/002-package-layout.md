# RFC-002: Package Layout

## The tree

Everything except the command entrypoint lives under `internal/`. The entrypoint stays thin — the frozen cobra command tree and a call into `app`.

```
cmd/hack-browser-data/        main; cobra commands and flags (the frozen CLI surface); flags → app.Request
internal/
  app/                        Validate, Discover, NewPlan, Execute, Report; dispatches on browser.Base to the base packages
  browser/                    Def, Base, Platform (Supported, Lookup); UserDataDir, DirID, Profile, DiscoverOptions;
                              Source, Extractor, ProfileSnapshot, Collector
    chromium/                 Chromium discovery, extractors, key input, archive sources, per-fork overrides (Yandex, Opera family)
    firefox/                  Firefox discovery, extractors, per-profile key derivation
    safari/                   Safari discovery and extractors
  category/                   Category: the data categories (the -c values and the output file names)
  record/                     record types (Login, Cookie, …), the sealed Record, Stamp; field names frozen by the output contract
  masterkey/                  Chromium key Set, Retriever, RetrieveAll, keys.json Dump; OS secret stores by build tag
  crypto/                     pure decrypt primitives over bytes (AES-CBC/GCM, 3DES, PBKDF2, ASN.1 PBE)
  snapshot/                   snapshot Dir and Snapshot, Capture, manifest, zip transport, the copier
  export/                     Writer; csv / json / cookie-editor encoders; --zip packaging
  sqlitex/                    thin modernc.org/sqlite helpers (open read-only by path, query)
  timeconv/                   browser epoch conversions (Chromium/WebKit, Unix, Firefox, Core Data)
  winapi/                     Windows only: syscall wrappers, DPAPI, locked-file copy, executable lookup
  abe/                        Windows only: App-Bound Encryption bridge — native/ C sources, payload/, injector/
  keychain/                   macOS only: the user's login keychain — one handle per run, opened on first use
  log/                        leveled logging (Debugf/Infof/Warnf)
```

The technique internals (the ABE payload, reflective injection, DPAPI and keychain specifics) sit behind `masterkey`, `keychain`, `winapi`, and the ABE unit by build tag. Their _mechanics_ are documented in the format and technique RFCs (013–021) and are out of scope for the design RFCs — here they are just packages with a contract ([RFC-006](006-key-retrieval-and-crypto.md)).

**The ABE unit lives in `internal/abe/`, as one unit.** `native/` holds the first-party C sources and `Makefile.frag` and no Go files, since Go rejects C files in a package that does not use cgo; `native/bootstrap/` is the layout package generated from them; `payload/` embeds the built `.bin`, which `make payload` writes there because `//go:embed` only reaches files in the embedding package's directory or below; and `injector/` loads it. `make payload` builds the payload and `make gen-layout` regenerates the layout package.

## Dependency direction

Dependencies point downward and are listed exhaustively; anything not listed is forbidden.

| Package                                                 | May import (project-internal)                                                                    |
| ------------------------------------------------------- | ------------------------------------------------------------------------------------------------ |
| `cmd/hack-browser-data`                                 | `app`, plus `browser` and `category` for help-text vocabulary (`-b` IDs, `-c` categories), `log` |
| `app`                                                   | every package below                                                                              |
| `browser/chromium`, `browser/firefox`, `browser/safari` | `browser`, `category`, `record`, `masterkey`, `crypto`, `snapshot`, `sqlitex`, `timeconv`, `log` |
| `export`                                                | `category`, `record`                                                                             |
| `browser`                                               | `category`, `record`, `masterkey`, `snapshot`                                                    |
| `masterkey`                                             | `crypto`, `winapi`, `abe`, `keychain`, `log`                                                     |
| `snapshot`                                              | `winapi`, `log`                                                                                  |
| `abe`                                                   | `winapi`, `log`                                                                                  |
| `record`                                                | `category`                                                                                       |
| `sqlitex`, `winapi`                                     | `log`                                                                                            |
| `category`, `crypto`, `timeconv`, `keychain`, `log`     | nothing                                                                                          |

`abe` stands for the ABE unit's Go packages, `abe/injector` and `abe/payload`.

Four rules keep the graph acyclic and its seams honest:

- **The base dispatch lives in `app`.** The base packages import `browser` for `Def`, `UserDataDir`, and `Extractor`, so `browser` cannot call them back; `app` switches on `Def.Base`. Init-time self-registration is ruled out by the `gochecknoinits` linter, and an explicit switch reads better on stage anyway.
- **`snapshot` speaks strings.** A `snapshot.File` names its user data dir by ID (`DirID`, a plain string) and its path relative to that directory; it never holds a `browser.UserDataDir`, because `browser.ProfileSnapshot` holds a `snapshot.Snapshot`.
- **`crypto` takes bytes.** Primitives receive raw keys and blobs; they never see a `masterkey.Set`, because `masterkey` is the package that calls them.
- **Interfaces live with their consumer.** `browser` declares the `Collector` its extractors use, and `app` implements it; `masterkey` and `browser/safari` each declare the one-method `LoginKeychain` they read. Constructors elsewhere return concrete types (`*snapshot.Dir`, `*export.Writer`).

Review keeps the table, not a tool: a change that needs an edge the table does not list — say, `crypto` importing `browser` — updates the table first, in the same pull request.

## Why `internal/` and nothing else (DECIDED)

v1.1.0 kept packages at the module root (`browser/`, `crypto/`, `filemanager/`, `log/`, `masterkey/`, `output/`, `types/`, `utils/...`), and its CLAUDE.md said "no `pkg/` + `internal/`." This design reverses that one decision and keeps the "keep it simple" spirit by using `internal/` _instead of_ a split, not in addition to one.

Reasons:

1. **No Go-API burden.** Anything importable by path becomes a de-facto public API that external users pin to. HBD has no intended library consumers — it is a CLI. `internal/` makes that explicit and compiler-enforced, so packages can be renamed and reshaped freely. The CLI contract ([RFC-000](000-goals-and-constraints.md)) is the only compatibility obligation, and it is unaffected.
2. **A clear seam.** `cmd/` is the only thing outside `internal/`. The boundary between "the program's entrypoint" and "the program" is visible in the tree.
3. **It is still flat.** We are not introducing `pkg/` as well. There is exactly one home for library code, and it is `internal/`.

Trade-off accepted: the module stops being importable as a library. The module path stays `github.com/moond4rk/hackbrowserdata` and releases continue as v1.x; there is no v2 (DECIDED).

## Package sizing and naming rules

- A package exists because it owns a _concept_, not because a file got long. `timeconv` and `category` are tiny on purpose. A package with nothing to own is not created — there is no `cli` (flags belong to `cmd`, rules to `app`) and no `textenc` (the CSV BOM is three bytes in `export`).
- A concept with per-OS implementations lives in one package with `_windows.go` / `_darwin.go` / `_linux.go` files; callers import one package and get the right implementation at build time. A concept that exists on only one OS — the locked-file copy, the ABE bridge, the macOS login keychain — lives in its own package whose files are all built for that OS: `winapi`, `abe`, and `keychain`.
- `winapi`, not `platform/windows`: a package named `windows` would collide with `golang.org/x/sys/windows`, which it imports, so every file would need an alias. `log` and `crypto` keep their v1.1.0 names although the standard library has packages of the same names: both are internal, and no file needs both.
- No `utils` grab-bag. The v1.1.0 tree was redistributed to named homes: `utils/sqliteutil` → `sqlitex`; `utils/fileutil` and `filemanager` → `snapshot` (copy, zip transport) and `export` (`--zip`); `utils/{winapi,winutil}` → `winapi`; `utils/injector` and `crypto/windows/` → `abe`, moved as one unit; `types` → `category`, `record`, and `browser`; `output` → `export`; `log` → `internal/log`.
