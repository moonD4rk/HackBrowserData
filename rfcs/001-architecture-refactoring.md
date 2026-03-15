# RFC-001: Architecture Refactoring

**Author**: moonD4rk
**Status**: Proposed
**Created**: 2025-09-01
**Updated**: 2026-03-15

## Abstract

This RFC addresses the overall architecture of HackBrowserData, focusing on areas NOT covered by RFC-002:

1. **Crypto layer**: cipher version detection, cross-platform algorithm differences, master key retrieval abstraction
2. **Browser registration & discovery**: declarative config, direct profile scanning
3. **CLI / library separation**: top-level `Run()` API
4. **Error handling**: structured errors with context

**Constraint**: Go 1.20 (Windows 7 support).

See RFC-002 for data extraction layer and file acquisition layer details.

---

## 1. Target Directory Structure

```
hackbrowserdata/
├── hackbrowserdata.go              # top-level library API: Run(), Option, Result
├── errors.go                       # structured ExtractionError type and sentinels
│
├── cmd/
│   └── hack-browser-data/
│       └── main.go                 # CLI entry point (thin shell over library API)
│
├── browser/
│   ├── browser.go                  # Browser interface, BrowserConfig, PickBrowsers()
│   ├── browser_darwin.go           # platformBrowsers() -> []BrowserConfig
│   ├── browser_windows.go          # platformBrowsers() -> []BrowserConfig
│   ├── browser_linux.go            # platformBrowsers() -> []BrowserConfig
│   │
│   ├── chromium/
│   │   ├── chromium.go             # Chromium struct, BrowsingData(), profile discovery
│   │   ├── chromium_darwin.go      # GetMasterKey() delegates to keyretriever
│   │   ├── chromium_windows.go     # GetMasterKey() delegates to keyretriever
│   │   ├── chromium_linux.go       # GetMasterKey() delegates to keyretriever
│   │   └── source.go              # chromiumSources: Category -> file paths mapping
│   │
│   ├── firefox/
│   │   ├── firefox.go              # Firefox struct, BrowsingData(), profile discovery
│   │   ├── firefox_test.go
│   │   └── source.go              # firefoxSources: Category -> file paths mapping
│   │
│   └── exploit/
│       └── gcoredump/
│           └── gcoredump.go        # CVE-2025-24204 macOS exploit (darwin only)
│
├── browserdata/
│   ├── browserdata.go              # BrowserData container struct, Output()
│   ├── outputter.go                # CSV/JSON output writer
│   ├── outputter_test.go
│   │
│   └── datautil/                   # shared helpers for extract methods
│       ├── sqlite.go              # QuerySQLite() helper
│       └── decrypt.go             # DecryptChromiumValue() helper
│
├── crypto/
│   ├── crypto.go                   # AESCBCDecrypt (renamed), AESGCMDecrypt, DES3, PKCS5
│   ├── crypto_darwin.go            # DecryptWithChromium (CBC), DecryptWithDPAPI (stub)
│   ├── crypto_windows.go           # DecryptWithChromium (GCM), DecryptWithDPAPI
│   ├── crypto_linux.go             # DecryptWithChromium (CBC), DecryptWithDPAPI (stub)
│   ├── crypto_test.go
│   ├── version.go                  # DetectVersion(), StripPrefix(), CipherVersion type
│   ├── asn1pbe.go                  # Firefox ASN.1 PBE key derivation
│   ├── asn1pbe_test.go
│   ├── pbkdf2.go                   # PBKDF2Key wrapper
│   │
│   └── keyretriever/              # master key retrieval abstraction
│       ├── keyretriever.go         # KeyRetriever interface, ChainRetriever
│       ├── keyretriever_darwin.go  # GcoredumpRetriever, SecurityCmdRetriever
│       ├── keyretriever_windows.go # DPAPIRetriever
│       ├── keyretriever_linux.go   # DBusRetriever, FallbackRetriever
│       └── params.go              # PBKDF2Params constants (saltysalt, iterations, etc.)
│
├── filemanager/
│   ├── session.go                  # Session: MkdirTemp, TempDir(), Acquire(), Cleanup()
│   └── acquirer.go                 # Acquirer interface, CopyAcquirer (with WAL/SHM)
│
├── types/
│   ├── category.go                 # Category enum: Password, Cookie, History, ...
│   ├── models.go                   # Data models: LoginEntry, CookieEntry, ...
│   └── types_test.go
│
├── log/
│   ├── log.go                      # global logger instance
│   ├── logger.go                   # Logger implementation
│   ├── logger_test.go
│   └── level/
│       └── level.go                # log level type
│
└── utils/
    ├── byteutil/
    │   └── byteutil.go
    ├── fileutil/
    │   ├── fileutil.go             # renamed from filetutil.go (typo fix)
    │   └── fileutil_test.go
    ├── typeutil/
    │   ├── typeutil.go
    │   └── typeutil_test.go
    └── chainbreaker/
        ├── chainbreaker.go
        └── chainbreaker_test.go
```

### What changed vs current structure

| Change | Current | Target |
|--------|---------|--------|
| **New** root-level API | — | `hackbrowserdata.go`, `errors.go` |
| **New** datautil helpers | — | `browserdata/datautil/` |
| **New** file manager | — | `filemanager/` |
| **New** key retriever | — | `crypto/keyretriever/` |
| **New** cipher version | — | `crypto/version.go` |
| **Restructured** types | `types/types.go` (22 DataType constants + file mappings) | `types/category.go` (9 Category constants) + `types/models.go` (data structs) |
| **Deleted** | `extractor/` package (interface + registry + factory) | no longer needed |
| **Deleted** | `browserdata/imports.go` | no longer needed |
| **Deleted** | `browserdata/localstorage/` + `browserdata/sessionstorage/` | merged into chromium/firefox extract methods |
| **Deleted** | `browser/consts.go` (27 constants) | inlined into `browser_*.go` configs |
| **Renamed** | `utils/fileutil/filetutil.go` (typo) | `utils/fileutil/fileutil.go` |
| **Renamed** | `AES128CBCDecrypt` | `AESCBCDecrypt` |

### Naming conventions

| Concept | Package | Type/Func | File |
|---------|---------|-----------|------|
| Data category | `types` | `Category` (int enum) | `category.go` |
| Data models | `types` | `LoginEntry`, `CookieEntry`, ... | `models.go` |
| Result container | `browserdata` | `BrowserData` (struct with typed slices) | `browserdata.go` |
| Browser config | `browser` | `BrowserConfig` | `browser.go` |
| Browser engine kind | `browser` | `BrowserKind` | `browser.go` |
| File source mapping | `chromium`/`firefox` | `source` (struct), `chromiumSources` (map) | `source.go` |
| Key retrieval | `keyretriever` | `KeyRetriever` (interface) | `keyretriever.go` |
| Strategy chain | `keyretriever` | `ChainRetriever` | `keyretriever.go` |
| macOS keychain | `keyretriever` | `SecurityCmdRetriever` | `keyretriever_darwin.go` |
| macOS exploit | `keyretriever` | `GcoredumpRetriever` | `keyretriever_darwin.go` |
| Windows DPAPI | `keyretriever` | `DPAPIRetriever` | `keyretriever_windows.go` |
| Linux D-Bus | `keyretriever` | `DBusRetriever` | `keyretriever_linux.go` |
| Linux fallback | `keyretriever` | `FallbackRetriever` | `keyretriever_linux.go` |
| PBKDF2 parameters | `keyretriever` | `PBKDF2Params` | `params.go` |
| Cipher version | `crypto` | `CipherVersion` (string) | `version.go` |
| Temp file session | `filemanager` | `Session` | `session.go` |
| File acquisition | `filemanager` | `Acquirer` (interface) | `acquirer.go` |
| Default acquirer | `filemanager` | `CopyAcquirer` | `acquirer.go` |
| SQLite helper | `datautil` | `QuerySQLite` (func) | `sqlite.go` |
| Decrypt helper | `datautil` | `DecryptChromiumValue` (func) | `decrypt.go` |
| Library options | root | `Option`, `Config` | `hackbrowserdata.go` |
| Library result | root | `Result` | `hackbrowserdata.go` |
| Structured error | root | `ExtractionError` | `errors.go` |

---

## 2. Core Data Model Redesign

### 2.1 Problem: MasterKey mixed with data types

The current `DataType` enum contains 22 constants that conflate three different concerns:

- **Infrastructure** (keys): `ChromiumKey`, `FirefoxKey4`
- **Browser engine prefix**: `ChromiumPassword` vs `FirefoxPassword` vs `YandexPassword`
- **File layout**: `Filename()`, `TempFilename()` methods on the enum

A password is a password regardless of which browser it came from. The browser engine determines *how* to extract it, not *what* it is.

### 2.2 New design: Category + Models

**`types/category.go`** — 9 data categories (down from 22 DataType constants):

```go
package types

type Category int

const (
    Password Category = iota
    Cookie
    Bookmark
    History
    Download
    CreditCard
    Extension
    LocalStorage
    SessionStorage
)

var AllCategories = []Category{
    Password, Cookie, Bookmark, History, Download,
    CreditCard, Extension, LocalStorage, SessionStorage,
}

func (c Category) String() string { ... }

func (c Category) IsSensitive() bool {
    switch c {
    case Password, Cookie, CreditCard:
        return true
    default:
        return false
    }
}
```

**`types/models.go`** — browser-agnostic data models, no encrypted fields:

```go
package types

import "time"

type LoginEntry struct {
    URL       string    `json:"url"        csv:"url"`
    Username  string    `json:"username"   csv:"username"`
    Password  string    `json:"password"   csv:"password"`
    CreatedAt time.Time `json:"created_at" csv:"created_at"`
}

type CookieEntry struct {
    Host       string    `json:"host"        csv:"host"`
    Path       string    `json:"path"        csv:"path"`
    Name       string    `json:"name"        csv:"name"`
    Value      string    `json:"value"       csv:"value"`
    IsSecure   bool      `json:"is_secure"   csv:"is_secure"`
    IsHTTPOnly bool      `json:"is_httponly"  csv:"is_httponly"`
    ExpireAt   time.Time `json:"expire_at"   csv:"expire_at"`
    CreatedAt  time.Time `json:"created_at"  csv:"created_at"`
}

type BookmarkEntry struct {
    Name      string    `json:"name"       csv:"name"`
    URL       string    `json:"url"        csv:"url"`
    Folder    string    `json:"folder"     csv:"folder"`
    CreatedAt time.Time `json:"created_at" csv:"created_at"`
}

type HistoryEntry struct {
    URL        string    `json:"url"         csv:"url"`
    Title      string    `json:"title"       csv:"title"`
    VisitCount int       `json:"visit_count" csv:"visit_count"`
    LastVisit  time.Time `json:"last_visit"  csv:"last_visit"`
}

type DownloadEntry struct {
    URL        string    `json:"url"         csv:"url"`
    TargetPath string    `json:"target_path" csv:"target_path"`
    TotalBytes int64     `json:"total_bytes" csv:"total_bytes"`
    StartTime  time.Time `json:"start_time"  csv:"start_time"`
    EndTime    time.Time `json:"end_time"    csv:"end_time"`
}

type CreditCardEntry struct {
    Name     string `json:"name"      csv:"name"`
    Number   string `json:"number"    csv:"number"`
    ExpMonth string `json:"exp_month" csv:"exp_month"`
    ExpYear  string `json:"exp_year"  csv:"exp_year"`
}

type StorageEntry struct {
    URL   string `json:"url"   csv:"url"`
    Key   string `json:"key"   csv:"key"`
    Value string `json:"value" csv:"value"`
}

type ExtensionEntry struct {
    Name        string `json:"name"        csv:"name"`
    ID          string `json:"id"          csv:"id"`
    Description string `json:"description" csv:"description"`
    Version     string `json:"version"     csv:"version"`
}
```

### 2.3 Result container

**`browserdata/browserdata.go`**:

```go
package browserdata

import "github.com/moond4rk/hackbrowserdata/types"

type BrowserData struct {
    Passwords      []types.LoginEntry
    Cookies        []types.CookieEntry
    Bookmarks      []types.BookmarkEntry
    Histories      []types.HistoryEntry
    Downloads      []types.DownloadEntry
    CreditCards    []types.CreditCardEntry
    Extensions     []types.ExtensionEntry
    LocalStorage   []types.StorageEntry
    SessionStorage []types.StorageEntry
}
```

No `Extractor` interface, no registry, no factory pattern. `BrowserData` is a plain struct with typed slices.

### 2.4 What was removed from types/

| Removed | Reason |
|---------|--------|
| `ChromiumKey`, `FirefoxKey4` | MasterKey is infrastructure, not data. Handled inside browser engine. |
| `Chromium*`/`Firefox*`/`Yandex*` prefixes | Browser engine is an extraction concern, not a type concern. |
| `Filename()`, `TempFilename()` methods | File layout is browser engine's internal knowledge. |
| `itemFileNames` map | Moved into `chromium/source.go` and `firefox/source.go`. |
| `DefaultChromiumTypes`, `DefaultFirefoxTypes`, `DefaultYandexTypes` | Replaced by `types.AllCategories`. |
| `extractor/` package | No longer needed — browser engines have typed extract methods. |

---

## 3. Crypto Layer

### 3.1 Current issues

Three platforms use completely different algorithms behind the same `DecryptWithChromium()` signature:

| Platform | File | Algorithm | IV/Nonce |
|----------|------|-----------|----------|
| Windows | `crypto/crypto_windows.go:17` | AES-256-GCM | 12-byte nonce from ciphertext |
| macOS | `crypto/crypto_darwin.go:9` | AES-128-CBC | hardcoded 16-byte space IV |
| Linux | `crypto/crypto_linux.go:5` | AES-128-CBC | hardcoded 16-byte space IV |

All three hardcode `ciphertext[3:]` to skip the "v10" prefix without checking the prefix.

Master key retrieval is scattered across three platform-specific `GetMasterKey()` methods with no shared abstraction.

### 3.2 Cipher version detection

**New file**: `crypto/version.go`

```go
package crypto

type CipherVersion string

const (
    CipherV10   CipherVersion = "v10"   // Chrome 80+
    CipherDPAPI CipherVersion = "dpapi"  // pre-Chrome 80, raw DPAPI
)

func DetectVersion(ciphertext []byte) CipherVersion {
    if len(ciphertext) >= 3 && string(ciphertext[:3]) == "v10" {
        return CipherV10
    }
    return CipherDPAPI
}

func StripPrefix(ciphertext []byte) []byte {
    if DetectVersion(ciphertext) == CipherV10 {
        return ciphertext[3:]
    }
    return ciphertext
}
```

### 3.3 Key retriever abstraction

**New package**: `crypto/keyretriever/`

```go
// keyretriever.go
type KeyRetriever interface {
    RetrieveKey(browserStorage string) ([]byte, error)
}

type ChainRetriever struct {
    retrievers []KeyRetriever
}

func NewChain(retrievers ...KeyRetriever) *ChainRetriever { ... }

func (c *ChainRetriever) RetrieveKey(storage string) ([]byte, error) {
    var lastErr error
    for _, r := range c.retrievers {
        key, err := r.RetrieveKey(storage)
        if err == nil && len(key) > 0 { return key, nil }
        lastErr = err
    }
    return nil, fmt.Errorf("all key retrievers failed: %w", lastErr)
}
```

Platform defaults:
- macOS: `NewChain(&GcoredumpRetriever{}, &SecurityCmdRetriever{})`
- Windows: `&DPAPIRetriever{}`
- Linux: `NewChain(&DBusRetriever{}, &FallbackRetriever{})`

**`params.go`** centralizes magic values:

```go
var (
    ChromiumMacOS = PBKDF2Params{Salt: []byte("saltysalt"), Iterations: 1003, KeyLen: 16}
    ChromiumLinux = PBKDF2Params{Salt: []byte("saltysalt"), Iterations: 1, KeyLen: 16}
)
```

---

## 4. Browser Registration & Discovery

### 4.1 Declarative browser config

```go
// browser/browser.go
type BrowserKind int
const (
    KindChromium BrowserKind = iota
    KindFirefox
)

type BrowserConfig struct {
    Key         string
    Name        string
    Kind        BrowserKind
    Storage     string
    UserDataDir string
    DataTypes   []types.Category
}

type Browser interface {
    Name() string
    BrowsingData(categories []types.Category) (*browserdata.BrowserData, error)
}
```

Platform files return `[]BrowserConfig`:

```go
// browser/browser_darwin.go
func platformBrowsers() []BrowserConfig {
    appSupport := homeDir + "/Library/Application Support"
    return []BrowserConfig{
        {Key: "chrome", Name: "Chrome", Kind: KindChromium, Storage: "Chrome",
         UserDataDir: appSupport + "/Google/Chrome"},
        // ...
        {Key: "firefox", Name: "Firefox", Kind: KindFirefox,
         UserDataDir: appSupport + "/Firefox/Profiles"},
    }
}
```

### 4.2 Unified PickBrowsers

```go
func PickBrowsers(name string, profile string) ([]Browser, error) {
    name = strings.ToLower(name)
    var browsers []Browser
    for _, cfg := range platformBrowsers() {
        if name != "all" && cfg.Key != name { continue }
        dir := cfg.UserDataDir
        if profile != "" { dir = profile }
        bs, err := newBrowserFromConfig(cfg, dir)
        if err != nil {
            log.Debugf("skip %s: %v", cfg.Name, err)
            continue
        }
        browsers = append(browsers, bs...)
    }
    return browsers, nil
}
```

### 4.3 Direct profile discovery (replace filepath.Walk)

Chromium profiles are deterministic (`Default/`, `Profile 1/`, ...). Directly enumerate and check known file paths instead of walking the entire directory tree.

Firefox profiles are `xxxxxxxx.name/` directories. Enumerate and check for known files like `key4.db` or `logins.json`.

---

## 5. CLI / Library Separation

**`hackbrowserdata.go`**:

```go
package hackbrowserdata

type Option func(*Config)
type Config struct {
    BrowserName string
    ProfilePath string
    FullExport  bool
}

func WithBrowser(name string) Option  { return func(c *Config) { c.BrowserName = name } }
func WithProfile(path string) Option  { return func(c *Config) { c.ProfilePath = path } }
func WithFullExport(v bool) Option    { return func(c *Config) { c.FullExport = v } }

type Result struct {
    BrowserName string
    Data        *browserdata.BrowserData
}

func Run(opts ...Option) ([]Result, error) { ... }
```

---

## 6. Error Handling

**`errors.go`**:

```go
package hackbrowserdata

type ExtractionError struct {
    Browser  string
    DataType string
    Op       string
    Err      error
}

func (e *ExtractionError) Error() string {
    return fmt.Sprintf("%s/%s: %s: %v", e.Browser, e.DataType, e.Op, e.Err)
}
func (e *ExtractionError) Unwrap() error { return e.Err }

var (
    ErrFileNotFound   = errors.New("file not found")
    ErrFileLocked     = errors.New("file locked by browser")
    ErrDecryptFailed  = errors.New("decryption failed")
    ErrKeyNotFound    = errors.New("master key not found")
    ErrUnsupportedVer = errors.New("unsupported encryption version")
)
```

---

## 7. Implementation Order

| Phase | Scope | Depends on |
|-------|-------|------------|
| **RFC-002 Phase 1** | `datautil/` helpers | — |
| **RFC-001 Phase 1** | `types/category.go` + `types/models.go` + `browserdata/browserdata.go` redesign | — |
| **RFC-001 Phase 2** | `crypto/version.go`, rename `AESCBCDecrypt` | — |
| **RFC-001 Phase 3** | `crypto/keyretriever/` | Phase 2 |
| **RFC-001 Phase 4** | Browser config + direct profile discovery | Phase 1 |
| **RFC-002 Phase 2** | `filemanager/` Session + Acquirer | RFC-001 Phase 1 |
| **RFC-001 Phase 5** | `hackbrowserdata.go`, `errors.go`, CLI separation | All above |

---

## 8. Relationship with RFC-002

| Area | RFC-001 (this doc) | RFC-002 |
|------|-------------------|---------|
| Data model redesign | covered | uses these types |
| Cipher version detection | covered | — |
| Master key retrieval | covered | — |
| Browser registration | covered | — |
| Profile discovery | covered | — |
| CLI separation | covered | — |
| Error types | covered | uses these types |
| File acquisition | — | covered |
| SQLite/decrypt helpers | — | covered |

---

## 9. Open Questions

1. **App-Bound Encryption (Chrome 127+)**: reserve extension points now or defer?
2. **Library API granularity**: is `Run()` sufficient, or do callers need per-data-type extraction?
3. **Firefox version detection**: is the key-length heuristic in `processMasterKey()` sufficient?
4. **Yandex special handling**: Yandex uses slightly different SQL queries and decryption. Keep as separate extract methods in `chromium/` or create a `yandex/` sub-package?

---

## References

- [Chromium OS Crypt](https://source.chromium.org/chromium/chromium/src/+/main:components/os_crypt/)
- [Chrome Password Decryption](https://github.com/chromium/chromium/blob/main/components/os_crypt/sync/os_crypt_win.cc)
- [Firefox NSS](https://developer.mozilla.org/en-US/docs/Mozilla/Projects/NSS)
