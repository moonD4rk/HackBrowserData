# RFC-013: CLI Redesign — Flat-Verb Surface & Cross-Host Restore

**Author**: moonD4rk
**Status**: Accepted — `archive` (#607) implemented; cross-platform `restore` (#606) pending
**Created**: 2026-06-03
**Revised**: 2026-06-06 (subdir-convention archive, dual-mode restore, Local State, delivery order)

## 1. Summary

The command-line surface has accreted two grammars: flat task-verbs (`dump`, `list`) alongside a noun-grouped `keys export` / `keys import` family. This RFC redesigns the whole surface around one grammar — flat verbs — and specifies the cross-host workflow end to end: export master keys on the origin host, archive the minimal data files, and restore (decrypt) them offline on an analyst host of any platform. It also records the structural problem the redesign exists to solve: the set of browsers a command can act on is platform-specific, yet cross-host restore must work for browsers that were never built for the analyst's OS. Breaking changes are accepted (pre-1.0); the method is CLI-first, deriving the internal data model backwards from the chosen surface.

## 2. Motivation

### 2.1 Two grammars in one tool

`dump` and `list` are flat task-verbs — the verb *is* the action. `keys export` / `keys import` is a noun-then-action grammar that implies a `keys` resource with sub-operations. Mixing the two in one small CLI reads as inconsistent: there is no `data export` or `browser list` to make the noun grammar systematic, so `keys` stands alone as a special case. The aim is a tool like kubectl — predictable because it is uniformly `verb resource` — but without kubectl's complexity. For a tool this size, the matching "simple and consistent" choice is uniformly flat verbs.

### 2.2 The cross-host workflow is half-automated

Cross-host decryption (export keys on the origin, decrypt a copied profile on the analyst host) shipped in #599–#605, but only the *keys* half is automated. The analyst still copies the origin's profile data by hand — awkward because the live SQLite files are locked on Windows, the full `User Data` tree is huge (caches), and only a handful of small files actually matter for decryption (#607).

### 2.3 Restore is bound to the local platform browser table (#606)

The consumer side reuses `DiscoverBrowsers`, which iterates `platformBrowsers()` — the browser table for the *analyst's* OS, selected by build tag (`browser_{darwin,windows,linux}.go`). A Windows-only fork such as Sogou / QQ / 360 lives only in the Windows table. On macOS, `restore -b sogou` matches nothing and aborts with `no browsers found`, even with a valid `keys.json` and the data supplied explicitly. This is the crux: the browsers a command may act on are platform-specific, but cross-host restore must transcend that — given a key and the data, it should decrypt any Chromium profile regardless of whether that browser exists on the analyst platform.

## 3. Proposed CLI surface

Six flat verbs, one grammar:

```
hack-browser-data [flags]                       # default → dump (also Windows double-click)
  dump      -b -c -f -d -p --zip                # local: decrypt this host's browsers → data
  dumpkeys  -b -o [--keychain-pw]               # origin: master keys → keys.json (stdout default)
  archive   -b -c -o                            # origin: minimal decryption-relevant files → zip
  restore   --keys K (--data-dir D | --data-zip Z) [-b] -c -f -d   # analyst: keys.json + data → decrypted
  list      [--detail]
  version
```

Workflows:

```
local     : hbd dump -b chrome -c cookie,password
cross-host: origin>   hbd dumpkeys -o keys.json
            origin>   hbd archive  -b chrome -o browser-data.zip
            analyst>  hbd restore --keys keys.json --data-zip browser-data.zip -c cookie
```

The `keys` parent command is removed: `keys export` becomes `dumpkeys`, `keys import` becomes `restore`, and a new `archive` fills the missing data-transport step. `dump` / `list` / `version` keep their current behavior; `dump` stays the default when no subcommand is given (which also covers the Windows double-click case).

## 4. The browser-universe model

The resolution to §2.3 is a single rule: **the set of browsers a command may act on — its "universe" — matches the nature of that command.**

| Command | Browser universe | `-b sogou` on macOS |
|---------|------------------|---------------------|
| `dump` / `dumpkeys` / `archive` / `list` | the local `platformBrowsers()` table (what this OS installs) | correctly fails — Sogou is not on macOS |
| `restore` | the `keys.json` itself (whatever the origin exported) | succeeds — the dump contains a Sogou vault |

`dump`, `dumpkeys`, `archive`, and `list` act on browsers *installed on this host*, so the platform table is the right source and `-b`'s vocabulary is the local set. `restore` acts on *transported artifacts that may have come from any platform*, so its universe is the `keys.json`, and `-b` validates against the dump's vaults, not the local table. Stated plainly: **the browsers you can restore are exactly the browsers in your `keys.json`.** This turns the platform difference from a bug into a property, and it is what makes #606 dissolve rather than be patched.

## 5. Cross-host artifacts and the restore command

The cross-host producer emits two independent, composable artifacts; the consumer takes both.

- `dumpkeys` writes `keys.json` — the portable master keys (stdout by default for `ssh origin hbd dumpkeys | …` pipelines; `-o` for a 0600 file).
- `archive` writes `browser-data.zip` — the decryption-relevant files for the requested `-c` categories (`Login Data`, `Cookies`, `Web Data`, `History`, …), read through the existing locked-file bypass. To carry more than one browser and to keep restore unambiguous, the zip is laid out as `<browser-key>/<User Data layout>` (e.g. `chrome/Default/Network/Cookies`) — one subdir per installation, each subdir being that browser's `User Data` root. Two things are always included regardless of `-c`: each profile's `Preferences`/`Preferences_02` (so restore can rediscover the profile — the marker is no extraction source) and the installation's `Local State` (carried for fidelity only; restore decrypts with the keys in `keys.json` and never reads it). Zip entry names are always forward-slash, so a Windows-produced archive restores on macOS/Linux.
- `restore` takes `--keys keys.json` and the data via two explicit flags, `--data-dir <dir>` or `--data-zip <zip>` (mutually exclusive, exactly one required). A zip is extracted to a temporary directory; a directory is used as-is, so `unzip browser-data.zip -d X && restore --data-dir X` equals `restore --data-zip browser-data.zip`. The data resolves two ways: when it holds `<browser-key>/` subdirs (the `archive` layout) each vault is rooted at its own subdir and several browsers restore at once; otherwise `--data-dir` is a single browser's hand-copied `User Data` root, which is unambiguous only for one vault — so `-b` must select it. This preserves the pre-redesign "point at a copied profile folder" workflow.

`restore` is a **separate verb**, not a `dump --keys` mode. Folding it into `dump` would force one command to carry two mutually-exclusive input modes (`-b` for local discovery xor `--keys/--data` for transported artifacts) and dead flags (a `--keychain-pw` that silently does nothing once keys are supplied — a friction the earlier `dump --keys` design already hit). One verb, one job keeps each command's flags and help self-contained. `restore -b` is an **optional filter** over the dump's vaults, not a required selector, because the dump self-describes what each vault is (§4, §6).

## 6. The cross-platform identity problem (#606): implementation options

Grounding facts:

- Every browser in the tables resolves to one of three engine kinds. All Windows-only forks (`360`, `360x`, `qq`, `sogou`, `dc`, `arc`, …) are `types.Chromium`; only Opera is `ChromiumOpera` and Yandex is `ChromiumYandex`. **Three kinds cover every fork.**
- The extraction logic (`sourcesForKind` / `extractorsForKind`) carries no build tags — it is OS-independent. A Sogou profile decrypts through the generic Chromium path with no Sogou-specific code.
- So restore needs only the **engine kind** (one of three) plus the data path and the keys. Everything else in `BrowserConfig` (display name, keychain label, ABE flag, default install path) is either a label or is used solely for *local* key derivation and discovery — all irrelevant once static keys and an explicit data path are supplied.

Two ways to give restore the kind for a browser absent from the analyst's table:

**Option A — self-describing dump (chosen).** The `keys.json` vault carries the kind. `restore` reads it and constructs a generic engine of that kind rooted at the supplied data path; it never consults `platformBrowsers()`. Minimal, and maximally robust: even a fork this build has never heard of by name still restores as long as its kind is one of the three.

**Option B — global, OS-independent browser registry.** Split the three per-OS tables into one full-fork registry (carrying kind) plus per-OS views that add paths and ABE flags. `-b`, help text, and `list` would then recognize every fork on any OS. This is a larger refactor and is not required for restore (restore always has a `keys.json` to serve as its universe); it is worth doing only if cross-platform `-b` / `list` awareness is a goal in its own right.

**Decision: Option A.** The `keys.json` vault carries the engine kind, and `restore` constructs from it without ever consulting `platformBrowsers()`. Option B above is the considered, rejected alternative.

This crystallizes the principle that lets cross-platform decryption and the current local mode coexist: **one engine constructor (`chromium.NewBrowser`), two config sources.** Local commands feed it configs from the per-OS `platformBrowsers()` table — unchanged; `restore` feeds it configs synthesized from the keys.json vaults. The cross-platform capability is an additive second source confined to the restore path, so the local mode is left untouched.

## 7. Downstream architecture implications (derived from the surface)

Working backwards from the chosen surface:

- **keydump struct** (`masterkey/dump.go`): the vault carries the engine kind so restore can construct without the local table. The `Browser` field becomes the canonical key (it was the display name), a `Kind` string field is added (values `chromium` / `chromium-yandex` / `chromium-opera`, mapped to/from the internal enum by an explicit bijection so a reordered enum can't silently corrupt), and `DumpVersion` is bumped to "2". The format is designed fresh — `ReadJSON` rejects other versions and there are no backward-compat shims for pre-redesign dumps. `UserDataDir` and `Profiles` remain informational. The keys stay `V10` / `V11` / `V20` (Chromium-only; Firefox keys are out of scope, §9).
- **`browser/keydump.go`**: `BuildDump` records the key and kind; the overlay `ApplyDump` (which mutates locally-discovered browsers) is replaced by `BuildFromDump`, which synthesizes a `BrowserConfig` per vault and builds the engine directly — no `platformBrowsers()` dependency. It resolves the data via the subdir convention or, for a hand-copied folder, the supplied dir as a single browser's root (§5). This is the mechanical form of §4.
- **`archive`** reuses the engine's per-category source resolution through a new `ArchiveSources` accessor — each source path is kept slash-canonical so the forward-slash zip entry name falls out directly — plus the existing locked-file session. The flattening `CompressDir` helper is unfit (it drops the layout and deletes the source), so `archive` uses a new layout-preserving `ZipDir`, and `restore --data-zip` a Zip-Slip-safe `Unzip`.
- **cmd layer**: drop the `keys` parent; add `dumpkeys`, `archive`, `restore` as siblings of `dump` / `list` / `version`.
- **Cross-cutting (orthogonal to the taxonomy)**: a Chromium-import password CSV format (`name,url,username,password,note`, #602) and category-aware credential prompting so a no-decryption request never asks for a password (#570).

## 8. Decisions (2026-06-03)

1. The browser-universe model (§4) is adopted: `restore`'s `-b` validates against the dump, not the local table.
2. #606 implementation: **Option A** — self-describing dump (§6).
3. keydump vault identity: **option 1A** — `Browser` becomes the canonical key and a `Kind` field is added (§7).
4. Verb names are final: `archive` and `restore`.

### Refinements (2026-06-06)

5. Archive layout is the subdir convention `<browser-key>/<User Data layout>` (multi-browser); `restore` is dual-mode — that layout, or a single hand-copied `User Data` root selected by `-b` (§5).
6. `archive` always includes each profile's `Preferences` marker (required for restore's profile discovery) and the installation's `Local State` (fidelity only — restore decrypts from `keys.json` and never reads it; §5).
7. No backward compatibility: the dump format is designed fresh, with no shims for pre-redesign artifacts.
8. Delivery order: `archive` (#607) lands first as an independent PR (it stands alone — its output also feeds the current overlay `restore` for same-OS browsers); the self-describing cross-platform `restore` (#606) follows.

## 9. Non-goals / deferred

- Firefox / Safari key export (Firefox keys are per-profile NSS; Safari has no portable key).
- A single self-describing bundle fusing keys + data into one file (the composable two-artifact model is chosen for now).
- Encrypted or signed dump artifacts.
- The global browser registry (§6 Option B), unless adopted for #606.

## Related RFCs

| RFC | Topic |
|-----|-------|
| [RFC-007](007-cli-and-output-design.md) | The CLI and output design this RFC revises |
| [RFC-003](003-chromium-encryption.md) | Cipher version dispatch (v10/v11/v20) consumed by restore |
| [RFC-006](006-key-retrieval-mechanisms.md) | Master-key retrieval the cross-host split externalizes |
| [RFC-001](001-project-architecture.md) | Browser interface and Extract() orchestration |
| [RFC-008](008-file-acquisition-and-platform-quirks.md) | Locked-file session and CompressDir used by archive |
