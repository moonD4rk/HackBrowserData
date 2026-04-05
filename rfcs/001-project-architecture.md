# RFC-001: Project Architecture & Data Model

**Author**: moonD4rk
**Status**: Living Document
**Created**: 2026-04-05

## 1. Project Positioning

HackBrowserData is a CLI security research tool that extracts and decrypts browser data from Chromium-based browsers and Firefox across Windows, macOS, and Linux.

Key constraints:

- **Go 1.20** — the module must build with Go 1.20 to maintain Windows 7 support. Features from Go 1.21+ (`log/slog`, `slices`, `maps`, `cmp`) must not be used.
- **Supported engines**: Chromium (including Yandex and Opera variants) and Firefox.
- **Supported platforms**: Windows (DPAPI), macOS (Keychain), Linux (D-Bus Secret Service).
- **No root-level library API** — the CLI calls `browser.PickBrowsers()` directly; there is no importable `pkg/` surface.

## 2. Directory Structure

```
HackBrowserData/
├── cmd/hack-browser-data/    # CLI entrypoint: cobra root, dump, list, version
├── browser/                  # Browser interface, PickBrowsers(), platform browser lists
│   ├── chromium/             # Chromium engine: extraction, decryption, profile discovery
│   └── firefox/              # Firefox engine: extraction, NSS key derivation
├── types/                    # Data model: Category enum, Entry structs, BrowserData
├── crypto/                   # Encryption primitives, cipher version detection
│   └── keyretriever/         # Platform-specific master key retrieval (Keychain/DPAPI/D-Bus)
├── filemanager/              # Temp file session, locked file handling (Windows)
├── output/                   # Output Writer: CSV, JSON, CookieEditor formatters
├── log/                      # Logging with level filtering
└── utils/                    # SQLite query helpers, file utilities
```

## 3. Core Data Model

### 3.1 Category

`Category` is an `int` enum representing 9 browser-agnostic data kinds: Password, Cookie, Bookmark, History, Download, CreditCard, Extension, LocalStorage, SessionStorage.

Three categories are classified as **sensitive** (Password, Cookie, CreditCard) via `IsSensitive()`, enabling safe-by-default export scenarios.

### 3.2 Entry Types

Each category has a corresponding Entry struct with `json` and `csv` struct tags. All structs are flat (no nesting) and use `time.Time` for timestamps.

| Struct | Category | Key Fields |
|--------|----------|------------|
| `LoginEntry` | Password | URL, Username, Password, CreatedAt |
| `CookieEntry` | Cookie | Host, Path, Name, Value, IsSecure, IsHTTPOnly, ExpireAt, CreatedAt |
| `BookmarkEntry` | Bookmark | Name, URL, Folder, CreatedAt |
| `HistoryEntry` | History | URL, Title, VisitCount, LastVisit |
| `DownloadEntry` | Download | URL, TargetPath, TotalBytes, StartTime, EndTime |
| `CreditCardEntry` | CreditCard | Name, Number, ExpMonth, ExpYear |
| `ExtensionEntry` | Extension | Name, ID, Description, Version |
| `StorageEntry` | LocalStorage, SessionStorage | URL, Key, Value |

`StorageEntry` is shared by both LocalStorage and SessionStorage.

### 3.3 BrowserData Container

`BrowserData` is the result container returned by `Extract()`. It holds typed slices — one per category. The container is populated field-by-field during extraction. The output layer uses `makeExtractor[T]()` generics to pull the correct slice for serialization.

## 4. Browser Interface & Registration

### 4.1 BrowserKind

Four engine kinds determine source paths and extractors:

| Kind | Description |
|------|-------------|
| `Chromium` | Standard Chromium layout |
| `ChromiumYandex` | Yandex variant: different file names and SQL queries |
| `ChromiumOpera` | Opera variant: different extension key, Roaming path on Windows |
| `Firefox` | Firefox: NSS encryption, SQLite + JSON files |

### 4.2 BrowserConfig

`BrowserConfig` is the declarative, platform-specific browser definition containing: Key (CLI matching), Name (display), Kind (engine), Storage (keychain label), UserDataDir (data path).

### 4.3 PickBrowsers() Flow

```
PickBrowsers(opts)
  → platformBrowsers()              // build-tagged: returns []BrowserConfig for this OS
  → pickFromConfigs(configs, opts)   // filter by name, apply profile-path/keychain overrides
      → newBrowsers(cfg)             // dispatch by Kind to chromium.NewBrowsers or firefox.NewBrowsers
          → discoverProfiles()       // scan for profile subdirectories
          → resolveSourcePaths()     // stat each candidate path, first match wins
```

Key design decisions:

- **One KeyRetriever per browser** — created once and shared across all profiles to prevent repeated keychain prompts on macOS.
- **Profile discovery differs by engine**: Chromium looks for `Preferences` files in subdirectories; Firefox accepts any subdirectory containing known source files.
- **Flat layout fallback** — Opera-style browsers that store data directly in UserDataDir (no profile subdirectories) are handled by falling back to the base directory.

### 4.4 Platform Browser Lists

Browser configs are defined per-platform via build tags:

- **macOS** — 12 browsers (Chrome, Edge, Chromium, Chrome Beta, Opera, OperaGX, Vivaldi, CocCoc, Brave, Yandex, Arc, Firefox)
- **Windows** — 16 browsers (all macOS minus Arc, plus 360 Speed, 360 Speed X, QQ, DC, Sogou)
- **Linux** — 8 browsers (Chrome, Edge, Chromium, Chrome Beta, Opera, Vivaldi, Brave, Firefox)

## 5. Extract() Orchestration

Both Chromium and Firefox engines follow the same extraction pattern:

```
Extract(categories)
  1. NewSession()               → create isolated temp directory
  2. acquireFiles(session)      → copy source files to temp dir (with dedup and WAL/SHM)
  3. getMasterKey(session)       → platform-specific key retrieval
  4. for each category:
       extractCategory(data, cat, masterKey, path)
  5. defer session.Cleanup()    → remove temp directory
```

For details on file acquisition, see [RFC-008](008-file-acquisition-and-platform-quirks.md). For encryption details, see [RFC-003](003-chromium-encryption.md) (Chromium) and [RFC-005](005-firefox-encryption.md) (Firefox). For key retrieval, see [RFC-006](006-key-retrieval-mechanisms.md).

### 5.1 Collect-and-Continue Pattern

The extraction loop maximizes data recovery. Each category is extracted independently — a failure in one does not affect others. Errors are handled at three levels:

| Level | Trigger | Action |
|-------|---------|--------|
| **Session failure** | Temp dir cannot be created | Abort entirely, return error |
| **Category failure** | Source file missing or extraction error | Skip category, continue to next |
| **Record failure** | Single row decryption fails | Skip record, continue extraction |

**Master key failure is non-fatal.** If the key cannot be retrieved, categories requiring decryption (passwords, cookies, credit cards) produce empty values, while non-encrypted categories (history, bookmarks, downloads) still succeed.

### 5.2 Custom Extractors

The `categoryExtractor` interface allows browser-specific extraction logic. Yandex and Opera use custom extractors for passwords and extensions respectively, while all other categories fall through to the default Chromium implementation.

## 6. Dependency Constraints

The module is pinned to `go 1.20` in `go.mod`. This is enforced by a CI lint check that fails if the directive changes.

| Dependency | Version | Purpose |
|-----------|---------|---------|
| `modernc.org/sqlite` | v1.31.1 (pinned) | Pure-Go SQLite. v1.32+ requires Go 1.21 |
| `github.com/syndtr/goleveldb` | v1.0.0 | LevelDB for Chromium localStorage/sessionStorage |
| `github.com/tidwall/gjson` | v1.18.0 | JSON path queries |
| `github.com/spf13/cobra` | v1.10.2 | CLI framework |
| `github.com/moond4rk/keychainbreaker` | v0.2.5 | macOS keychain decryption |
| `github.com/godbus/dbus/v5` | v5.2.2 | Linux D-Bus Secret Service |
| `golang.org/x/sys` | v0.27.0 | Windows syscalls (DPAPI, DuplicateHandle) |

## Related RFCs

| RFC | Topic |
|-----|-------|
| [RFC-002](002-chromium-data-storage.md) | Chromium data file locations and storage formats |
| [RFC-003](003-chromium-encryption.md) | Chromium encryption mechanisms per platform |
| [RFC-004](004-firefox-data-storage.md) | Firefox data file locations and storage formats |
| [RFC-005](005-firefox-encryption.md) | Firefox NSS encryption and key derivation |
| [RFC-006](006-key-retrieval-mechanisms.md) | Platform-specific master key retrieval |
| [RFC-007](007-cli-and-output-design.md) | CLI commands and output formats |
| [RFC-008](008-file-acquisition-and-platform-quirks.md) | File acquisition and platform quirks |
| [RFC-009](009-windows-locked-file-bypass.md) | Windows locked file bypass technique |
