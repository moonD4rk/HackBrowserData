# RFC-011: Testing & Quality

The pipeline split ([RFC-001](001-architecture-and-pipeline.md)) exists partly so each concern can be tested in isolation, and the compatibility contract ([RFC-000](000-goals-and-constraints.md)) only holds if a test pins it. This RFC says how.

## Test strategy by layer

| Layer                                             | How it's tested                                                                                                                                                                                                                                                                                                                  |
| ------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `crypto` primitives                               | Pure functions over bytes; fixed known-answer vectors. No OS, no files.                                                                                                                                                                                                                                                          |
| `category`                                        | `String` / `Parse` round-trip over `All()`; the zero `Category` is "none".                                                                                                                                                                                                                                                       |
| `record`                                          | `Stamp` sets `Browser` and `Profile`; every record type's tags equal snake_case(field name); each type's `Category()` matches the table in [RFC-003](003-core-types.md).                                                                                                                                                         |
| extractors (`browser/*`)                          | A sanitized fixture (SQLite, JSON, plist, LevelDB) in `testdata/`, captured into a `t.TempDir()` snapshot, parsed, and compared to a golden record set. No live browser.                                                                                                                                                         |
| `snapshot`                                        | `t.TempDir()`; capture with `-wal`/`-shm` sidecars, reopen, zip round-trip, Zip-Slip rejection; a vanished file reads back as `fs.ErrNotExist` and a failed copy as its error while the rest of the capture completes; `Close` removes a temporary snapshot directory — also after an error — and leaves an opened one in place. |
| `export`                                          | Records in → golden `csv` / `json` / `cookie-editor` out; deterministic order; byte-identical to today's encoder on the same records, HTML escaping included.                                                                                                                                                                    |
| `app` (planner/report)                            | Table tests over synthetic user data dirs; asserts the `Plan` with `reflect.DeepEqual`, every outcome rule, and the exit status.                                                                                                                                                                                                 |
| `masterkey`                                       | Fake retrievers: per version the first key wins and later retrievers are not called; a version's error appears only when all its retrievers fail; a `v20` failure keeps the `v10` key.                                                                                                                                           |
| platform (`winapi`, OS retrievers in `masterkey`) | `_windows_test.go` / `_darwin_test.go` / `_linux_test.go`, run on the real OS in CI; the ABE path also on a dedicated Windows host.                                                                                                                                                                                              |

Principle: **a test needs a fixture file, not a running browser.** Because every parser reads through a `ProfileSnapshot` over a `snapshot.Snapshot`, a `testdata/` file is a complete input. This is what makes the extractor suite fast and runnable on any OS.

## Compatibility tests

These guard the contract; a failure means the rewrite changed something users can see.

- **CLI surface.** For every command, the set of flags — name, shorthand, type, default — is compared against a golden captured from v1.1.0. One golden serves every OS and is checked on all three: none of these attributes varies by OS — only `-b`'s usage text does, and usage text is not recorded. Only the additive `--keychain-pw-stdin` is expected on top.
- **Output parity.** During the migration, the old and the new code paths run on the same fixture snapshots and their output directories are diffed byte for byte, for every format; the fixtures include URLs with `&` and `<`, so JSON escaping is covered. The only expected differences are the listed output fixes ([RFC-008](008-output-and-export.md)), each pinned by its own test.
- **`keys.json` and archives.** A v1.1.0 `keys.json` and archive restore with the new binary; a new `keys.json` parses with v1.1.0's reader (version `"2"`, unknown fields ignored); a new archive keeps the `<browser-key>/…` layout.
- **`list --detail` counts.** Counts from Extract-without-secrets equal today's per-category counts on the same fixtures.

## Golden files

Export and extractor tests compare against checked-in golden outputs. Deterministic ordering is what makes goldens stable — without it, goldens would flap. A `-update` flag regenerates them on intentional changes. Goldens double as documentation of the exact output shape, useful for the README and for talks.

The compatibility goldens — the flag sets, every output format, `keys.json`, and the archive layout — live in the repository-root `testdata/` and are read by the tests in `cmd/hack-browser-data`, the one package that never moves, so every phase compares its output against the same files. After the switch-over, Phase 8 moves each test and its goldens into the package it covers ([RFC-012](012-migration-roadmap.md)). A golden holds what no other golden already implies: the Cookie-Editor golden holds only `cookie.json`, because every other category is asserted to match the json output byte for byte, and `restore` is asserted to match the extract output except for its `browser` stamp.

## Regression tests for the known bugs

Each bug designed out by this rewrite gets a test that would have caught it, so it can never silently return:

- **Firefox bookmark fields** (`folder` always `url`, `type` empty, `id` 0): golden assertions on a fixture with nested folders.
- **Firefox history order**: `visit_count` descending for every browser, Firefox included.
- **Safari bookmark IDs**: unique across the whole tree, not repeated per folder.
- **Arc multi-dir**: two `arc` user data dirs yield IDs `arc` and `arc-2`, distinct snapshots, and distinct `keys.json` vaults, while overrides and the vaults' `browser` key still use `arc`.
- **`--zip` data loss**: a failing zip write leaves every output file in place.
- **Late flag validation**: `-f bogus` exits `1` with no discovery call and no password resolution (both injected as spies).
- **Eager secrets**: `dump -b chrome -c history` never calls `ProfileSnapshot.MasterKeys` and never opens the login keychain: a counting fake `PasswordFunc` stays at zero.
- **JSON escaping**: a URL with `&` and `<` is written escaped in `json` and unescaped in `cookie-editor`, as today.
- **Supported browsers**: `Supported()` lists only this OS's rows, and `Lookup` finds every row, so a vault for a Windows-only fork restores on macOS.
- **macOS key order**: when the first `v10` strategy fails and a later one succeeds, the key is used and no error is reported.
- **One keychain report**: with a locked login keychain, every macOS Chromium user data dir falls through to the next `v10` retriever, and the reason is reported once for the run, not once per browser.
- **Unset outcome**: a `Result` whose outcome was never set does not report as `ok`.
- **Per-dir snapshots**: a dump over two user data dirs never has both dirs' copies on disk at once.
- **`-p` validation**: a non-existent `-p` path exits `1`, not `0` with "no browsers found".
- **`-p` with `-b all`**: a warning is logged and no user data dir is overridden.
- **`-p` at a Chromium profile directory**: its parent becomes the user data dir, so `Local State` is captured and keys resolve.
- **`-p` for Firefox**: only the named profile is exported, not every sibling under it.
- **`-p` for Safari**: rejected in Validate with a reason.
- **Keychain password precedence**: explicit flag > prompt > none.
- **Temp snapshot cleanup**: a run that fails mid-way leaves no copied `Login Data` or `Cookies` behind.
- **Discovery errors are not absence**: an unreadable user data dir (permission denied) is reported as an `error` result, distinct from one that simply isn't there (nothing). The home directory comes from `DiscoverOptions`, so a `t.TempDir()` stands in.
- **Safari discovery reads a copy**: listing Safari profiles opens a snapshot copy of `SafariTabs.db`, never the live file.

## Table and payload consistency

- Every key in the Chromium override table resolves through `browser.Lookup` to a Chromium `Def`.
- Every Windows row of the table with `ABE` other than `ABENone` has a row in the payload's `com_iid.c` with the same executable and ABE kind; the test parses the C table, which may list browsers the Go table does not, such as Avast. It lives inside `browser` and reads the whole table, since `Supported()` lists only the running OS's rows. (The Go-side `winutil.Table` join, and the `TestWinUtilTableCoversABEBrowsers` that guarded it, disappear into `Platform`.)

## Import direction

The table in [RFC-002](002-package-layout.md) is kept by review, not by a linter or a test: every new import edge is either in the table or added to it in the same pull request. The compiler still rejects cycles. Until the switch-over, today's packages — and `cmd`, which still drives them — also import the new packages they are moving onto; those edges leave with the old code ([RFC-012](012-migration-roadmap.md)).

## Go 1.20 pitfalls

The `go 1.20` directive constrains more than the standard library:

- **Loop variables are shared.** Under `go 1.20`, every closure created in a loop sees the loop variable's last value, and the per-iteration semantics of Go 1.22 apply only to modules that declare `go 1.22` or later, whatever toolchain builds them. Extractors are closures built per profile, and `go vet`'s `loopclosure` check only looks at closures started with `go`, `defer`, and the like, so it does not cover them. A two-profile fixture in every base asserts that each extraction reads its own profile's files.
- **No 1.21+ helpers.** `sync.OnceValues`, `slices`, `maps`, `cmp`, and the `min` / `max` / `clear` builtins are unavailable: the per-dir `MasterKeys` call-once and the keychain handle each hold a `sync.Once` with stored results, and sorting uses `sort.SliceStable` with a multi-key less function.

## Tooling

- **golangci-lint v2** — the project's existing config, extended with `tagliatelle` (`case.use-field-name: true`, `case.rules: {json: snake, csv: snake}`) on the packages that own a wire format — `record`, `masterkey` (`keys.json`), and `snapshot` (`manifest.json`); the Cookie-Editor encoder and any struct that mirrors a browser's own file format stay outside it. `lll` stays at 140.
- **gofumpt** + **goimports** with `-local github.com/moond4rk/hackbrowserdata`.
- **CI matrix**: Go pinned to `1.20.x` (not `go-version-file`) on Windows, macOS, and Linux; build + vet + lint + test; a step that fails unless `go.mod` says `go 1.20` with no `toolchain` line. The Windows ABE path is additionally validated on a dedicated Windows host against the current cookie/key baseline, not in hosted CI.

## Doc hygiene

The project CLAUDE.md is updated as part of the migration ([RFC-012](012-migration-roadmap.md)):

- it references `go-sqlmock`, which `go.mod` doesn't have — the real strategy is `t.TempDir()` + fixtures;
- it says HBD supports Chromium-based browsers and Firefox — Safari is missing, as in the root help text;
- it points at `crypto/abe_embed_windows.go` / `crypto/abe_stub_windows.go`, which now live in `crypto/windows/payload/`;
- its "`golang.org/x/text` will be removed" note is moot (the module is already indirect-only) and its "no `pkg/` + `internal/`" rule is superseded by [RFC-002](002-package-layout.md).

In the code, the `pickTest` / `TestPickFromConfigs` names outlived the function they test (`discoverFromConfigs`) and go away with it. The README says `cookie-editor` skips non-cookie categories, while the code writes them as standard JSON; the contract freezes the code's behavior, so the README is corrected. The format and technique RFCs (013–021) keep today's type names until Phase 8 moves them to the new ones ([RFC-012](012-migration-roadmap.md)). No RFC citations go into source comments — traceability stays in git and these documents.

## Coverage intent

Coverage is a signal, not a target. The intent: every extractor has at least one golden fixture, every outcome has a planner test, every designed-out bug has a regression test, and every frozen surface has a compatibility test. Those guarantees matter more than a percentage.
