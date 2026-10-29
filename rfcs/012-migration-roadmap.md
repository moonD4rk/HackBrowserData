# RFC-012: Migration Roadmap

## One integration branch

The rewrite is built on the integration branch `refactor/main-v2`, cut from `main`, on the existing module path and with no v2. Every step is a pull request into that branch, squash-merged once CI is green, so each step becomes one commit whose title carries its PR number. `main` is frozen meanwhile — no Go changes and no bot commits land there — and keeps shipping v1.1.x from today's code.

When Phase 8 is done, a final pull request from `refactor/main-v2` to `main` runs the full CI, and `main` is fast-forwarded to the branch, so every step commit keeps its order, its date, and its PR number. If `main` moved after all, the branch is first rebased onto it with `--committer-date-is-author-date`, building and testing every replayed commit. GitHub's "Rebase and merge" button is not used for that final step: it rewrites every commit's committer and date.

Inside the branch the rewrite is still incremental: new `internal/` packages grow beside the current code, one base is wired end to end before the others follow, and old code is deleted only once its replacement is tested. Because the compatibility contract ([RFC-000](000-goals-and-constraints.md)) freezes the CLI and the output, "stable" is checkable: at no commit does a command line, an output file, a `keys.json`, or an archive change outside the listed fixes. Every commit builds and passes the compatibility suite, so `git bisect` works across the whole rewrite once it is on `main`.

## Phase order

Leaves first, bottom-up along the dependency arrows ([RFC-002](002-package-layout.md)); then one vertical slice through the whole pipeline; then the rest of the browsers. A pure bottom-up order would first exercise the new stack end to end in its last phase, after weeks of adapters; the slice moves that moment to the middle.

### Phase 0 — Scaffold and guardrails

- Make `rfcs/` the one design record: the design RFCs (000–012) and the format and technique RFCs (013–021), indexed by `rfcs/README.md`; the RFCs they supersede are removed.
- Add `tagliatelle` ([RFC-011](011-testing-and-quality.md)), so every new record, `keys.json`, and manifest field keeps its snake_case wire name from day one.
- Pin CI to Go `1.20.x` and fail on any `go` directive other than `1.20` or on a `toolchain` line.
- Capture the compatibility goldens from today's code: the per-command CLI flag sets first; then output-parity fixtures for every format (with URLs that exercise JSON escaping), and sample `keys.json` files and archives.
- No behavior change. CLAUDE.md's "no `pkg/` + `internal/`" note is updated to reflect the `internal/`-only decision.

### Phase 1 — Leaves

- Move the utility code into named homes: `utils/sqliteutil` → `sqlitex`, the per-browser epoch helpers → `timeconv`, `log` → `internal/log` (name kept).
- These have no project-internal dependencies beyond `log`, so they move with mechanical refactors and their existing tests.

### Phase 2 — `category`, `record`, and `export`

- Introduce `internal/category` — `Category` with bare values numbered from 1, today's `String()` values, `All` and `Parse` — and `internal/record`: `Login`, `Cookie`, and the other record types with flat `Browser` + `Profile` fields followed by today's field names and order, set through `record.Stamp` ([RFC-003](003-core-types.md)).
- Apply the naming in [RFC-004](004-naming-decisions.md) verbatim: `types.Category` → `category.Category`, `types.LoginEntry` → `record.Login` and every other `…Entry` → its bare name, no `Category` prefix, no `Owner`/`Origin` umbrella.
- Move `output` to `internal/export` — `Writer` and the `csv`, `json`, and `cookie-editor` encoders — on the new record types ([RFC-008](008-output-and-export.md)). It depends only on `category`, `record`, and `log`.
- Move the row order out of the extractors into the exporter ([RFC-008](008-output-and-export.md)), so an extractor emits rows in source order. This lands the Firefox history order fix with its regression test.
- No other output change: `tagliatelle` and the parity suite must stay green.

### Phase 3 — `crypto`, `masterkey`, `keychain`, `winapi`

- Make `crypto` pure primitives; move OS secret-store work into `masterkey` — retrievers per OS, `RetrieveAll` with first-key-wins per version and independent versions, `Version` as a string type, `DecryptDPAPI`, the `keys.json` `Dump` unchanged at version `"2"`, `ABEKind` ([RFC-006](006-key-retrieval-and-crypto.md)). No technique is rewritten — only the boundary is drawn.
- Add `internal/keychain` (macOS only): the login keychain handle, opened on first use and at most once per run. The macOS `v10` retriever reads through it from this phase, the Safari base when it is ported (Phase 6); today's separate password check and per-consumer keychain opens go away at the switch-over (Phase 7), once both read through it ([RFC-006](006-key-retrieval-and-crypto.md)).
- Fold `utils/{winapi,winutil}` into `internal/winapi`. The ABE unit — `utils/injector` and `crypto/windows/` with its C sources, generated `bootstrap` package, and `payload` embed package, plus `Makefile.frag` — stays where it is; only its importers change. Its move to `internal/abe/` is DEFERRED ([RFC-002](002-package-layout.md)).

### Phase 4 — `snapshot`

- Build `internal/snapshot` ([RFC-007](007-snapshot-and-acquisition.md)): `Dir` (`Temp`, `Open`, `Unzip`, `Capture`, `Snapshot`, `Zip`, `Close`) and `Snapshot.Path` with the per-file error contract, the copier with the Windows locked-file fallback and SQLite sidecars, user-data-dir IDs in the layout, and `0700` temp snapshot directories removed on every exit path.
- Replace `otiai10/copy` with the snapshot copier ([RFC-010](010-dependencies.md)).
- `archive` and `restore` move onto the snapshot; the archive-layout and `keys.json` compatibility tests guard both directions.

### Phase 5 — Vertical slice: the Chromium base end to end

- Add the `browser.Def` table with the Chromium rows, `browser.ProfileSnapshot` and the per-dir `MasterKeys` call-once in `app` (with the run's login keychain added to the `Locator` on macOS), the Chromium base package (`Discover`, `Extractors`, `KeyLocator`, `ArchiveSources`, the Yandex and Opera overrides), and the `app` stages — `Validate`, `Discover`, `NewPlan`, `Execute`, `Report` ([RFC-001](001-architecture-and-pipeline.md)).
- The new path runs beside today's; on the fixtures its output must equal the Phase 0 goldens byte for byte, and the Windows host compares it against the cookie/key baseline.

### Phase 6 — Widen: Firefox and Safari

- Port the Firefox and Safari bases one category at a time, each with a `testdata/` fixture and golden.
- Land the remaining listed output fixes here — Firefox bookmark fields and Safari bookmark IDs — each with its regression test and as an expected diff in the parity suite.
- With the Firefox base, a profile whose key cannot be recovered exports its logins instead of stopping the run ([RFC-008](008-output-and-export.md)); the goldens do not change.

### Phase 7 — Switch over

- `dump`, `dumpkeys`, `archive`, `restore`, and `list` run on the pipeline.
- `archive` writes the snapshot `manifest.json` and `restore` reads it as its `Discovery` ([RFC-007](007-snapshot-and-acquisition.md)): writer and reader land together, so the round trip is tested end to end, and every earlier archive stays byte-identical to today's.
- Validate-first ordering, `--keychain-pw-stdin` with the new precedence, the `-p` fixes, the safe `--zip` order, and the `restore` stamp fix ([RFC-009](009-cli-surface.md)).
- Yandex logins and cards export without their data key, like every other store, so `list --detail` counts them without a key ([RFC-008](008-output-and-export.md)).

### Phase 8 — Delete & document

- Remove the superseded top-level packages — `browser/`, `crypto/` except the ABE unit's `crypto/windows/`, `filemanager/`, `log/`, `masterkey/`, `output/`, `types/`, and `utils/` except `utils/injector` — once nothing references them.
- Move the compatibility tests out of `cmd/hack-browser-data` into the packages they cover — the export goldens into `export`, `keys.json` into `masterkey`, the archive into `snapshot`, the extract goldens into `browser/chromium`, and `restore` into `app` — each with its goldens in its own `testdata/`; `cmd` keeps only the flag-surface test, and the root `testdata/` goes away.
- Update CLAUDE.md (the new paths), the README (Safari, the `cookie-editor` behavior), and the root help text.
- Bring the RFCs to their finished state: the format and technique RFCs (013–021) move from today's type names — `BrowserConfig`, `WindowsABE`, `winutil.Table`, `Hints`, the `Retrievers` struct — to the names in [RFC-004](004-naming-decisions.md); the design RFCs describe the code as it is instead of comparing it with today's; and this roadmap is removed.
- Final `go mod tidy` under go@1.20; confirm the `go` directive is still `1.20` and `otiai10/copy` and `mint` are gone.

## Guardrails per phase

- Every phase keeps `go build ./cmd/hack-browser-data/` green on Go 1.20.x across all three OSes.
- Every phase keeps the compatibility suite green: the same flag sets, byte-identical output except for the listed fixes, and readable `keys.json` and archives in both directions.
- Every phase is one or more reviewable PRs into `refactor/main-v2`, each with its own tests; no PR depends on a later one to compile.
- The Windows ABE and locked-file smoke test runs at the end of Phases 3, 4, 5, and 7 — the phases that can affect key retrieval or acquisition — against the current cookie/key baseline, on the dedicated Windows host.

## What ships when

Nothing from the rewrite ships before it reaches `main`. Until then `main` keeps releasing v1.1.x from today's code; after the fast-forward, the next release is cut from the rewritten tree. Because the CLI and the output are frozen, users see no difference except the listed fixes.
