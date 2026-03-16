# RFC-001: Architecture Refactoring

**Author**: moonD4rk
**Status**: Proposed
**Created**: 2025-09-01
**Updated**: 2026-03-16

## Abstract

This RFC addresses the overall architecture of HackBrowserData:

1. **Data model redesign**: `Category` enum + browser-agnostic `*Entry` structs
2. **Crypto layer**: cipher version detection, master key retrieval abstraction
3. **Browser registration & discovery**: declarative config, direct profile scanning
4. **Yandex variant handling**: source overrides + query overrides
5. **Error handling**: collect-and-continue pattern

**Constraint**: Go 1.20 (Windows 7 support).

See RFC-002 for file acquisition, extract method details, and output.

---

## 1. Target Directory Structure

```
hackbrowserdata/
├── cmd/
│   └── hack-browser-data/
│       └── main.go                    # CLI: flag parsing → PickBrowsers → BrowsingData → Output
│
├── browser/
│   ├── browser.go                     # Browser interface, BrowserKind, BrowserConfig, PickBrowsers()
│   ├── browser_darwin.go              # platformBrowsers() → []BrowserConfig
│   ├── browser_windows.go             # platformBrowsers() → []BrowserConfig
│   ├── browser_linux.go               # platformBrowsers() → []BrowserConfig
│   │
│   ├── chromium/
│   │   ├── chromium.go                # Chromium struct, BrowsingData()
│   │   ├── chromium_darwin.go         # GetMasterKey() → delegates to keyretriever
│   │   ├── chromium_windows.go        # GetMasterKey() → delegates to keyretriever
│   │   ├── chromium_linux.go          # GetMasterKey() → delegates to keyretriever
│   │   ├── source.go                  # chromiumSources, yandexSources maps
│   │   ├── extract_password.go        # extractPasswords() + default SQL query
│   │   ├── extract_cookie.go          # extractCookies() + default SQL query
│   │   ├── extract_history.go         # extractHistories() + default SQL query
│   │   ├── extract_download.go        # extractDownloads() + default SQL query
│   │   ├── extract_bookmark.go        # extractBookmarks() (JSON)
│   │   ├── extract_creditcard.go      # extractCreditCards() + default SQL query
│   │   ├── extract_extension.go       # extractExtensions() (JSON)
│   │   └── extract_storage.go         # extractLocalStorage(), extractSessionStorage() (LevelDB)
│   │
│   ├── firefox/
│   │   ├── firefox.go                 # Firefox struct, BrowsingData(), deriveMasterKey()
│   │   ├── firefox_test.go
│   │   ├── source.go                  # firefoxSources map
│   │   ├── extract_password.go        # extractPasswords() (JSON + ASN1PBE)
│   │   ├── extract_cookie.go          # extractCookies() (SQLite, no encryption)
│   │   ├── extract_history.go         # extractHistories() (SQLite)
│   │   ├── extract_download.go        # extractDownloads() (SQLite)
│   │   ├── extract_bookmark.go        # extractBookmarks() (SQLite)
│   │   ├── extract_extension.go       # extractExtensions() (JSON)
│   │   └── extract_storage.go         # extractLocalStorage() (SQLite)
│   │
│   └── exploit/
│       └── gcoredump/
│           └── gcoredump.go           # CVE-2025-24204 macOS exploit (darwin only)
│
├── browserdata/
│   ├── browserdata.go                 # BrowserData struct (typed slices)
│   ├── output.go                      # BrowserData.Output() — CSV/JSON writer
│   ├── output_test.go
│   │
│   └── datautil/
│       ├── sqlite.go                  # QuerySQLite() helper
│       ├── query.go                   # queryRows[T]() generic helper (Go 1.20)
│       └── decrypt.go                 # DecryptChromiumValue() helper
│
├── crypto/
│   ├── crypto.go                      # AESCBCDecrypt, AESGCMDecrypt, DES3, PKCS5
│   ├── crypto_darwin.go               # DecryptWithChromium (CBC), DecryptWithDPAPI (returns error)
│   ├── crypto_windows.go              # DecryptWithChromium (GCM), DecryptWithDPAPI
│   ├── crypto_linux.go                # DecryptWithChromium (CBC), DecryptWithDPAPI (returns error)
│   ├── crypto_test.go
│   ├── version.go                     # DetectVersion(), StripPrefix(), CipherVersion
│   ├── asn1pbe.go                     # Firefox ASN.1 PBE key derivation
│   ├── asn1pbe_test.go
│   ├── pbkdf2.go
│   │
│   └── keyretriever/
│       ├── keyretriever.go            # KeyRetriever interface, ChainRetriever
│       ├── keyretriever_darwin.go     # GcoredumpRetriever, SecurityCmdRetriever
│       ├── keyretriever_windows.go    # DPAPIRetriever
│       ├── keyretriever_linux.go      # DBusRetriever, FallbackRetriever
│       └── params.go                  # PBKDF2Params (saltysalt, iterations)
│
├── filemanager/
│   └── session.go                     # Session: MkdirTemp, TempDir(), Acquire(), Cleanup()
│
├── types/
│   ├── category.go                    # Category enum (9 values)
│   ├── models.go                      # LoginEntry, CookieEntry, ... (browser-agnostic)
│   └── types_test.go
│
├── log/
│   ├── log.go
│   ├── logger.go
│   ├── logger_test.go
│   └── level/
│       └── level.go
│
└── utils/
    ├── byteutil/
    │   └── byteutil.go
    ├── fileutil/
    │   ├── fileutil.go                # renamed from filetutil.go
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
| **New** `browserdata/datautil/` | — | SQLite + decrypt helpers |
| **New** `filemanager/` | — | Session-based temp file management |
| **New** `crypto/keyretriever/` | — | Master key retrieval abstraction |
| **New** `crypto/version.go` | — | Cipher version detection |
| **New** `browser/chromium/extract_*.go` | — | Per-category extract methods |
| **New** `browser/firefox/extract_*.go` | — | Per-category extract methods |
| **New** `browser/*/source.go` | — | File source mapping per engine |
| **Restructured** `types/` | 22 DataType constants + file mappings | 9 Category constants + data model structs |
| **Deleted** `extractor/` | interface + registry + factory | not needed |
| **Deleted** `browserdata/imports.go` | init() side-effect registration | not needed |
| **Deleted** `browserdata/password/`, `cookie/`, etc. | 9 sub-packages | extract logic moved into browser engines |
| **Deleted** `browser/consts.go` | 27 scattered constants | inlined into BrowserConfig |
| **Renamed** `filetutil.go` | typo | `fileutil.go` |
| **Renamed** `AES128CBCDecrypt` | misleading name | `AESCBCDecrypt` |

### Naming conventions

| Concept | Package | Type/Func | File |
|---------|---------|-----------|------|
| Data category | `types` | `Category` (int enum) | `category.go` |
| Data models | `types` | `LoginEntry`, `CookieEntry`, ... | `models.go` |
| Result container | `browserdata` | `BrowserData` | `browserdata.go` |
| Browser config | `browser` | `BrowserConfig` | `browser.go` |
| Browser engine kind | `browser` | `BrowserKind` | `browser.go` |
| File source mapping | `chromium`/`firefox` | `source` struct, `chromiumSources` map | `source.go` |
| Key retrieval | `keyretriever` | `KeyRetriever` (interface) | `keyretriever.go` |
| Strategy chain | `keyretriever` | `ChainRetriever` | `keyretriever.go` |
| Cipher version | `crypto` | `CipherVersion` | `version.go` |
| Temp file session | `filemanager` | `Session` | `session.go` |
| SQLite helper | `datautil` | `QuerySQLite` (func) | `sqlite.go` |
| Generic query helper | `datautil` | `queryRows[T]` (func) | `query.go` |
| Decrypt helper | `datautil` | `DecryptChromiumValue` (func) | `decrypt.go` |

### File naming convention for `extract_*.go`

Files inside `browser/chromium/` and `browser/firefox/` use the `extract_` prefix for extraction logic. This groups them visually when sorted alphabetically:

```
chromium.go                 ← struct + BrowsingData orchestration
chromium_darwin.go          ← platform: master key
chromium_linux.go
chromium_windows.go
extract_bookmark.go         ← extract: one file per Category
extract_cookie.go
extract_creditcard.go
extract_download.go
extract_extension.go
extract_history.go
extract_password.go
extract_storage.go
source.go                   ← file source mapping
```

Three natural groups: `chromium*` (struct + platform), `extract_*` (data extraction), `source.go` (file mapping). Each `extract_*.go` file contains the default SQL query constant and the extract method (~20-30 lines).

---

## 2. Core Data Model Redesign

### 2.1 Problem: MasterKey mixed with data types

The current `DataType` enum contains 22 constants that conflate three concerns:

- **Infrastructure** (keys): `ChromiumKey`, `FirefoxKey4`
- **Browser engine prefix**: `ChromiumPassword` vs `FirefoxPassword` vs `YandexPassword`
- **File layout**: `Filename()`, `TempFilename()` methods on the enum

A password is a password regardless of which browser it came from. The browser engine determines *how* to extract, not *what* the data is.

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

func NonSensitiveCategories() []Category {
    var cats []Category
    for _, c := range AllCategories {
        if !c.IsSensitive() {
            cats = append(cats, c)
        }
    }
    return cats
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

### 2.4 What was removed from types/

| Removed | Reason |
|---------|--------|
| `ChromiumKey`, `FirefoxKey4` | MasterKey is infrastructure, handled inside browser engine |
| `Chromium*`/`Firefox*`/`Yandex*` prefixes | Browser engine is extraction concern, not type concern |
| `Filename()`, `TempFilename()` | File layout is browser engine's internal knowledge |
| `itemFileNames` map | Moved into `chromium/source.go` and `firefox/source.go` |
| `DefaultChromiumTypes`, `DefaultFirefoxTypes`, `DefaultYandexTypes` | Replaced by `types.AllCategories` |
| `extractor/` package | No longer needed — browser engines have typed extract methods |
| `browserdata/imports.go` | No longer needed — no init() registration |

---

## 3. Crypto Layer

### 3.1 Cipher version detection

**New file**: `crypto/version.go`

```go
type CipherVersion string

const (
    CipherV10   CipherVersion = "v10"   // Chrome 80+
    CipherV20   CipherVersion = "v20"   // Chrome 127+ App-Bound Encryption
    CipherDPAPI CipherVersion = "dpapi"  // pre-Chrome 80
)

func DetectVersion(ciphertext []byte) CipherVersion {
    if len(ciphertext) < 3 { return CipherDPAPI }
    prefix := string(ciphertext[:3])
    switch prefix {
    case "v10":
        return CipherV10
    case "v20":
        return CipherV20
    default:
        return CipherDPAPI
    }
}

func StripPrefix(ciphertext []byte) []byte {
    ver := DetectVersion(ciphertext)
    if ver == CipherV10 || ver == CipherV20 {
        return ciphertext[3:]
    }
    return ciphertext
}
```

Version-specific post-processing (e.g., v20 cookie value has a 32-byte header) belongs here, not in extract methods:

```go
// DecryptCookieValue handles version-specific cookie decryption.
func DecryptCookieValue(key, ciphertext []byte) ([]byte, error) {
    version := DetectVersion(ciphertext)
    payload := StripPrefix(ciphertext)

    switch version {
    case CipherV10:
        return decryptPayload(key, payload)
    case CipherV20:
        value, err := decryptPayload(key, payload)
        if err != nil { return nil, err }
        if len(value) > 32 {
            return value[32:], nil  // strip App-Bound header
        }
        return value, nil
    default:
        return nil, fmt.Errorf("unsupported cipher version: %s", version)
    }
}
```

### 3.2 Key retriever abstraction

**New package**: `crypto/keyretriever/`

```go
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

**`params.go`** centralizes PBKDF2 magic values with source links:

```go
var (
    // https://source.chromium.org/chromium/chromium/src/+/master:components/os_crypt/os_crypt_mac.mm
    ChromiumMacOS = PBKDF2Params{Salt: []byte("saltysalt"), Iterations: 1003, KeyLen: 16}
    // https://source.chromium.org/chromium/chromium/src/+/main:components/os_crypt/os_crypt_linux.cc
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
    KindChromiumYandex  // Chromium variant with different file names and SQL queries
    KindFirefox
)

type BrowserConfig struct {
    Key         string          // lookup key: "chrome", "firefox"
    Name        string          // display name: "Chrome", "Firefox"
    Kind        BrowserKind
    Storage     string          // keychain label (macOS/Linux)
    UserDataDir string          // e.g. ~/Library/Application Support/Google/Chrome/
}

type Browser interface {
    Name() string
    BrowsingData(categories []types.Category) (*browserdata.BrowserData, error)
}
```

### 4.2 Unified PickBrowsers

```go
func PickBrowsers(name, profile string) ([]Browser, error) {
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

func newBrowserFromConfig(cfg BrowserConfig, dir string) ([]Browser, error) {
    switch cfg.Kind {
    case KindChromium, KindChromiumYandex:
        return chromium.New(cfg, dir)
    case KindFirefox:
        return firefox.New(dir)
    default:
        return nil, fmt.Errorf("unknown browser kind: %d", cfg.Kind)
    }
}
```

### 4.3 Direct profile discovery

Chromium profiles are deterministic (`Default/`, `Profile 1/`, ...). Directly `os.ReadDir()` and check known file paths instead of `filepath.Walk`.

Firefox profiles are `xxxxxxxx.name/` directories. Enumerate and check for `key4.db` or `logins.json`.

---

## 5. Yandex Variant Handling

Yandex is Chromium-based with 3 differences:

| Aspect | Standard Chromium | Yandex |
|--------|------------------|--------|
| Password file | `Login Data` | `Ya Passman Data` |
| Password SQL | `SELECT origin_url, ...` | `SELECT action_url, ...` |
| CreditCard file | `Web Data` | `Ya Credit Cards` |

### 5.1 Separate source map

```go
// browser/chromium/source.go

var yandexSources = map[types.Category]source{
    types.Password:       {paths: []string{"Ya Passman Data"}},        // different
    types.Cookie:         {paths: []string{"Network/Cookies", "Cookies"}},
    types.History:        {paths: []string{"History"}},
    types.Download:       {paths: []string{"History"}},
    types.Bookmark:       {paths: []string{"Bookmarks"}},
    types.CreditCard:     {paths: []string{"Ya Credit Cards"}},        // different
    types.Extension:      {paths: []string{"Secure Preferences"}},
    types.LocalStorage:   {paths: []string{"Local Storage/leveldb"}, isDir: true},
    types.SessionStorage: {paths: []string{"Session Storage"}, isDir: true},
}
```

### 5.2 Query overrides (default + override pattern)

Each extract method defines its own default SQL query constant. The Chromium struct holds an optional override map:

```go
// browser/chromium/chromium.go
type Chromium struct {
    name           string
    storage        string
    profileDir     string
    sources        map[types.Category]source    // chromiumSources or yandexSources
    queryOverrides map[types.Category]string    // nil for standard Chromium
    keyRetriever   keyretriever.KeyRetriever
}

var yandexQueryOverrides = map[types.Category]string{
    types.Password: `SELECT action_url, username_value, password_value, date_created FROM logins`,
}
```

Extract methods check for overrides locally:

```go
// browser/chromium/extract_password.go
const defaultLoginQuery = `SELECT origin_url, username_value, password_value, date_created FROM logins`

func (c *Chromium) extractPasswords(masterKey []byte, path string) ([]types.LoginEntry, error) {
    query := defaultLoginQuery
    if q, ok := c.queryOverrides[types.Password]; ok {
        query = q
    }
    // ... rest of extraction
}
```

### 5.3 Wiring at creation time

```go
func New(cfg browser.BrowserConfig, userDataDir string) ([]*Chromium, error) {
    sources := chromiumSources
    var overrides map[types.Category]string
    if cfg.Kind == browser.KindChromiumYandex {
        sources = yandexSources
        overrides = yandexQueryOverrides
    }
    // ... discover profiles, create Chromium instances with sources + overrides
}
```

Zero if-branches in any extract method. All variant differences concentrated in `source.go` and `New()`.

---

## 6. Error Handling

### 6.1 Collect-and-continue pattern

`BrowsingData()` collects errors per category but continues extracting. The returned `data` and `err` can both be non-nil:

```go
func (c *Chromium) BrowsingData(categories []types.Category) (*browserdata.BrowserData, error) {
    session, err := filemanager.NewSession()
    if err != nil { return nil, err }
    defer session.Cleanup()

    files := c.acquireFiles(session, categories)
    masterKey, err := c.keyRetriever.RetrieveKey(c.storage)
    if err != nil { return nil, err }  // fatal: can't decrypt anything

    data := &browserdata.BrowserData{}
    var errs []error

    for _, cat := range categories {
        path, ok := files[cat]
        if !ok { continue }

        switch cat {
        case types.Password:
            data.Passwords, err = c.extractPasswords(masterKey, path)
        case types.Cookie:
            data.Cookies, err = c.extractCookies(masterKey, path)
        case types.History:
            data.Histories, err = c.extractHistories(path)
        case types.Download:
            data.Downloads, err = c.extractDownloads(path)
        case types.Bookmark:
            data.Bookmarks, err = c.extractBookmarks(path)
        case types.CreditCard:
            data.CreditCards, err = c.extractCreditCards(masterKey, path)
        case types.Extension:
            data.Extensions, err = c.extractExtensions(path)
        case types.LocalStorage:
            data.LocalStorage, err = c.extractLocalStorage(path)
        case types.SessionStorage:
            data.SessionStorage, err = c.extractSessionStorage(path)
        }
        if err != nil {
            log.Debugf("extract %s: %v", cat, err)
            errs = append(errs, fmt.Errorf("%s: %w", cat, err))
        }
    }
    return data, errors.Join(errs...)  // Go 1.20
}
```

### 6.2 Error severity levels

| Level | Behavior | Example |
|-------|----------|---------|
| Session/key failure | `return nil, err` — abort entirely | Disk full, keychain denied |
| Category failure | Log, skip, continue next category | Cookie file locked |
| Single record failure | Skip record, continue extraction | One cookie decryption failed |

### 6.3 Caller pattern

```go
data, err := b.BrowsingData(categories)
if err != nil {
    log.Warnf("%s: %v", b.Name(), err)  // partial failure
}
if data == nil {
    continue  // total failure
}
data.Output(dir, b.Name(), format)  // output whatever succeeded
```

---

## 7. Implementation Order

| Phase | Scope | Risk |
|-------|-------|------|
| 1 | `types/category.go` + `types/models.go` + `browserdata/browserdata.go` | Zero — new files only |
| 2 | `browserdata/datautil/sqlite.go` + `decrypt.go` | Zero — new files only |
| 3 | `crypto/version.go`, rename `AESCBCDecrypt` | Low — internal crypto changes |
| 4 | `crypto/keyretriever/` | Low — new package |
| 5 | `browser/chromium/source.go` + `extract_*.go` | Medium — new extract methods |
| 6 | `browser/firefox/source.go` + `extract_*.go` | Medium — new extract methods |
| 7 | `filemanager/session.go` | Low — new package |
| 8 | Wire `BrowsingData()` + `BrowserConfig` + `PickBrowsers()` | High — connects everything |
| 9 | Delete old code: `extractor/`, `browserdata/*/`, `imports.go` | High — removal |
| 10 | Update CLI, tests, cross-platform build verification | Medium |

---

## 8. Relationship with RFC-002

| Area | RFC-001 (this doc) | RFC-002 |
|------|-------------------|---------|
| Data model (Category + *Entry) | defines | uses |
| BrowserData container | defines | implements Output |
| Cipher version | covered | — |
| Master key retrieval | covered | — |
| Browser registration | covered | — |
| Yandex variant | covered | — |
| Error handling pattern | covered | — |
| BrowsingData() orchestration | covered | — |
| File source mapping | — | covered |
| File acquisition (Session) | — | covered |
| Extract method details | — | covered |
| datautil helpers | — | covered |
| Output implementation | — | covered |

---

## 9. Open Questions

1. **App-Bound Encryption (Chrome 127+ v20)**: `crypto/version.go` has the extension point. Implementation deferred until tested.
2. **Firefox version detection**: is the key-length heuristic in `processMasterKey()` sufficient, or formalize it?
3. **Sort direction**: standardize all categories to DESC by date? (Firefox history/download currently ASC)

---

## References

- [Chromium OS Crypt](https://source.chromium.org/chromium/chromium/src/+/main:components/os_crypt/)
- [Chrome Password Decryption](https://github.com/chromium/chromium/blob/main/components/os_crypt/sync/os_crypt_win.cc)
- [Firefox NSS](https://developer.mozilla.org/en-US/docs/Mozilla/Projects/NSS)
