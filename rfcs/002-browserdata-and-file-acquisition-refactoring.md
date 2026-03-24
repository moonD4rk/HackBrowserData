# RFC-002: Data Extraction & File Acquisition

**Author**: moonD4rk
**Status**: Proposed
**Created**: 2026-03-14
**Updated**: 2026-03-22

## Abstract

This RFC covers the implementation details of data extraction and file acquisition:

1. **File source mapping**: how each browser engine maps categories to files
2. **File acquisition**: Session-based temp file management with deduplication
3. **Extract methods**: concrete implementations for each data category
4. **Shared helpers**: `QuerySQLite()` and `DecryptChromiumValue()`
5. **Output**: writing `Extract` results to CSV/JSON files

**Constraint**: Go 1.20 (Windows 7 support).

See RFC-001 for data model (`Category` + `*Entry` types), crypto layer, browser registration, and Yandex variant design.

---

## 1. Data Flow

```
CLI: main.go
    │
    ▼
browser.PickBrowsers("all", "")
    │
    │  platformBrowsers() → []Config
    │  → chromium.New(cfg, dir) / firefox.New(dir)
    ▼
Browser.Extract(categories)
    │
    ├─ filemanager.NewSession()
    │   └─ acquireFiles() with dedup → map[Category]tempPath
    │
    ├─ masterKey
    │   Chromium: keyretriever.RetrieveKey(storage)
    │   Firefox:  deriveMasterKey(key4dbPath)
    │
    └─ per-category extract methods
        ├─ c.extractPasswords(masterKey, path) → []LoginEntry
        ├─ c.extractCookies(masterKey, path)   → []CookieEntry
        ├─ c.extractHistories(path)            → []HistoryEntry
        ├─ c.extractDownloads(path)            → []DownloadEntry
        ├─ c.extractBookmarks(path)            → []BookmarkEntry
        ├─ c.extractCreditCards(masterKey, path) → []CreditCardEntry
        ├─ c.extractExtensions(path)           → []ExtensionEntry
        ├─ c.extractLocalStorage(path)         → []StorageEntry    (LevelDB)
        └─ c.extractSessionStorage(path)       → []StorageEntry    (LevelDB)
            │
            ▼
        browserdata.BrowserData{Passwords: [...], Cookies: [...], ...}
            │
            ▼
        BrowserData.Output(dir, name, format)
            │
            ▼
        chrome_default_password.csv
        chrome_default_cookie.json
        ...
```

---

## 2. File Source Mapping

### 2.1 Category → source (one flat map per engine)

```go
// browser/chromium/source.go

type source struct {
    paths []string // candidates in priority order
    isDir bool
}

var chromiumSources = map[types.Category]source{
    types.Password:       {paths: []string{"Login Data"}},
    types.Cookie:         {paths: []string{"Network/Cookies", "Cookies"}},
    types.History:        {paths: []string{"History"}},
    types.Download:       {paths: []string{"History"}},       // same file, different query
    types.Bookmark:       {paths: []string{"Bookmarks"}},
    types.CreditCard:     {paths: []string{"Web Data"}},
    types.Extension:      {paths: []string{"Secure Preferences"}},
    types.LocalStorage:   {paths: []string{"Local Storage/leveldb"}, isDir: true},
    types.SessionStorage: {paths: []string{"Session Storage"}, isDir: true},
}
```

```go
// browser/firefox/source.go

var firefoxSources = map[types.Category]source{
    types.Password:     {paths: []string{"logins.json"}},
    types.Cookie:       {paths: []string{"cookies.sqlite"}},
    types.History:      {paths: []string{"places.sqlite"}},
    types.Download:     {paths: []string{"places.sqlite"}},   // same file
    types.Bookmark:     {paths: []string{"places.sqlite"}},   // same file
    types.Extension:    {paths: []string{"extensions.json"}},
    types.LocalStorage: {paths: []string{"webappsstore.sqlite"}},
}
```

Yandex source map defined in RFC-001 Section 5.

### 2.2 File acquisition with deduplication

When multiple categories map to the same file (e.g. History + Download), the file is copied once:

```go
func (c *Chromium) acquireFiles(session *filemanager.Session, categories []types.Category) map[types.Category]string {
    result := make(map[types.Category]string)
    copied := make(map[string]string) // abs src → temp dst

    for _, cat := range categories {
        src, ok := c.sources[cat]  // uses c.sources (chromiumSources or yandexSources)
        if !ok { continue }

        for _, rel := range src.paths {
            abs := filepath.Join(c.profileDir, rel)

            if dst, ok := copied[abs]; ok {
                result[cat] = dst  // reuse already-copied file
                break
            }

            dst := filepath.Join(session.TempDir(), filepath.Base(rel))
            if err := session.Acquire(abs, dst, src.isDir); err == nil {
                copied[abs] = dst
                result[cat] = dst
                break
            }
        }
    }
    return result
}
```

### 2.3 Firefox key4.db: infrastructure, not a Category

Each Firefox profile has its own `key4.db`. The master key is derived once in `New()` and stored on the struct, so `Extract()` never re-derives it:

```go
// firefox.New() — called once per profile
func New(profileDir string) (*Firefox, error) {
    // derive master key from this profile's key4.db
    keyPath := filepath.Join(profileDir, "key4.db")
    masterKey, err := deriveMasterKey(keyPath)
    if err != nil { return nil, err }

    return &Firefox{
        profileDir: profileDir,
        masterKey:  masterKey,
        sources:    firefoxSources,
    }, nil
}

func (f *Firefox) Extract(categories []types.Category) (*browserdata.BrowserData, error) {
    session, _ := filemanager.NewSession()
    defer session.Cleanup()

    files := f.acquireFiles(session, categories)

    // masterKey was derived in New() from this profile's key4.db
    data := &browserdata.BrowserData{}
    // ... extract each category using f.masterKey ...
}
```

### 2.4 Profile Discovery

Profile discovery functions are pure helpers (no struct receiver) that scan the filesystem:

```go
// profile/finder.go

// discoverProfiles returns sub-directory names that look like Chrome profiles.
// Matches "Default" or any name starting with "Profile ".
// Falls back to ["."] for Opera-style layouts (data files live directly in userDataDir).
func discoverProfiles(userDataDir string) []string {
    entries, err := os.ReadDir(userDataDir)
    if err != nil { return []string{"."} }

    var profiles []string
    for _, e := range entries {
        if !e.IsDir() { continue }
        name := e.Name()
        if name == "Default" || strings.HasPrefix(name, "Profile ") {
            profiles = append(profiles, name)
        }
    }
    if len(profiles) == 0 {
        return []string{"."}
    }
    return profiles
}

// discoverDataFiles checks which categories have actual data files in profileDir.
func discoverDataFiles(profileDir string, sources map[types.Category]source) map[types.Category]string {
    found := make(map[types.Category]string)
    for cat, src := range sources {
        for _, rel := range src.paths {
            abs := filepath.Join(profileDir, rel)
            info, err := os.Stat(abs)
            if err != nil { continue }
            if src.isDir && !info.IsDir() { continue }
            if !src.isDir && info.IsDir() { continue }
            found[cat] = abs
            break
        }
    }
    return found
}

// isValidBrowserDir checks whether the directory belongs to a real browser install.
// Chromium: requires "Local State" file. Firefox: requires directory existence.
func isValidBrowserDir(dir string, kind BrowserKind) bool {
    switch kind {
    case KindChromium, KindChromiumYandex:
        _, err := os.Stat(filepath.Join(dir, "Local State"))
        return err == nil
    case KindFirefox:
        info, err := os.Stat(dir)
        return err == nil && info.IsDir()
    }
    return false
}
```

**Testing approach**: all three functions are pure filesystem operations, easily testable with `t.TempDir()`:

```go
func TestDiscoverProfiles(t *testing.T) {
    dir := t.TempDir()
    os.MkdirAll(filepath.Join(dir, "Default"), 0o755)
    os.MkdirAll(filepath.Join(dir, "Profile 1"), 0o755)
    os.MkdirAll(filepath.Join(dir, "System Profile"), 0o755)

    profiles := discoverProfiles(dir)
    assert.Equal(t, []string{"Default", "Profile 1"}, profiles)
}

func TestDiscoverDataFiles(t *testing.T) {
    dir := t.TempDir()
    os.WriteFile(filepath.Join(dir, "Login Data"), []byte{}, 0o644)
    os.MkdirAll(filepath.Join(dir, "Network"), 0o755)
    os.WriteFile(filepath.Join(dir, "Network", "Cookies"), []byte{}, 0o644)

    files := discoverDataFiles(dir, chromiumSources)
    assert.Contains(t, files, types.Password)
    assert.Contains(t, files, types.Cookie)
}

func TestAcquireFiles_Dedup(t *testing.T) {
    dir := t.TempDir()
    os.WriteFile(filepath.Join(dir, "History"), []byte("data"), 0o644)

    session, _ := filemanager.NewSession()
    defer session.Cleanup()

    c := &Chromium{profileDir: dir, sources: chromiumSources}
    files := c.acquireFiles(session, []types.Category{types.History, types.Download})
    assert.Equal(t, files[types.History], files[types.Download])
}
```

### 2.5 Platform Config Example

Each platform file returns the full list of known browsers with their `UserDataDir` paths:

```go
// browser/browser_windows.go
func platformBrowsers() []Config {
    return []Config{
        {Key: "chrome",   Name: "Chrome",        Kind: KindChromium, UserDataDir: homeDir + "/AppData/Local/Google/Chrome/User Data"},
        {Key: "edge",     Name: "Microsoft Edge", Kind: KindChromium, UserDataDir: homeDir + "/AppData/Local/Microsoft/Edge/User Data"},
        {Key: "opera",    Name: "Opera",          Kind: KindChromium, UserDataDir: homeDir + "/AppData/Roaming/Opera Software/Opera Stable"},
        {Key: "yandex",   Name: "Yandex",         Kind: KindChromiumYandex, UserDataDir: homeDir + "/AppData/Local/Yandex/YandexBrowser/User Data"},
        {Key: "firefox",  Name: "Firefox",        Kind: KindFirefox,  UserDataDir: homeDir + "/AppData/Roaming/Mozilla/Firefox/Profiles"},
    }
}
```

`PickBrowsers()` iterates this list, calls `isValidBrowserDir()` to skip browsers that aren't installed, then calls `discoverProfiles()` to find all profiles within valid browser directories.

---

## 3. Shared Helpers: `utils/sqliteutil/`

### 3.1 SQLite query helper

```go
// utils/sqliteutil/sqlite.go

func QuerySQLite(dbPath string, journalOff bool, query string, scanFn func(*sql.Rows) error) error {
    db, err := sql.Open("sqlite", dbPath)
    if err != nil { return err }
    defer db.Close()

    if journalOff {
        if _, err := db.Exec("PRAGMA journal_mode=off"); err != nil { return err }
    }

    rows, err := db.Query(query)
    if err != nil { return err }
    defer rows.Close()

    for rows.Next() {
        if err := scanFn(rows); err != nil {
            log.Debugf("scan row error: %v", err)
            continue  // skip bad row, continue extraction
        }
    }
    return rows.Err()
}
```

### 3.2 Generic query helper — `datautil/query.go`

```go
package sqliteutil

// queryRows is a generic helper (Go 1.20) that wraps QuerySQLite
// and collects results into a typed slice. Each extract method
// only needs to provide the scan function.
func QueryRows[T any](path string, journalOff bool, query string, scanRow func(*sql.Rows) (T, error)) ([]T, error) {
    var items []T
    err := QuerySQLite(path, journalOff, query, func(rows *sql.Rows) error {
        item, err := scanRow(rows)
        if err != nil { return nil } // skip bad row
        items = append(items, item)
        return nil
    })
    return items, err
}
```

### 3.3 Chromium decrypt helper

Moved to `browser/chromium/decrypt.go` as an unexported function `decryptValue()`. It is Chromium-specific (DPAPI → AES-GCM/CBC fallback) and only used by Chromium extract methods. See RFC-001 for details.

---

## 4. Extract Method Examples

Each extract method lives in its own `extract_*.go` file inside the browser engine package (see RFC-001 for naming convention). The default SQL query is a `const` in the same file. Override is checked via `c.queryOverrides`.

### 4.1 Chromium password (SQLite + decryption)

```go
// browser/chromium/extract_password.go

const defaultLoginQuery = `SELECT origin_url, username_value, password_value, date_created FROM logins`

func (c *Chromium) extractPasswords(masterKey []byte, path string) ([]types.LoginEntry, error) {
    logins, err := sqliteutil.QueryRows(path, false, c.query(types.Password),
        func(rows *sql.Rows) (types.LoginEntry, error) {
            var url, username string
            var pwd []byte
            var created int64
            if err := rows.Scan(&url, &username, &pwd, &created); err != nil {
                return types.LoginEntry{}, err
            }
            password, _ := decryptValue(masterKey, pwd)
            return types.LoginEntry{
                URL:       url,
                Username:  username,
                Password:  string(password),
                CreatedAt: typeutil.TimeEpoch(created),
            }, nil
        })
    if err != nil { return nil, err }

    sort.Slice(logins, func(i, j int) bool {
        return logins[i].CreatedAt.After(logins[j].CreatedAt)
    })
    return logins, nil
}
```

### 4.2 Chromium cookie (SQLite + decryption)

```go
// browser/chromium/extract_cookie.go

const defaultCookieQuery = `SELECT name, encrypted_value, host_key, path,
    creation_utc, expires_utc, is_secure, is_httponly,
    has_expires, is_persistent FROM cookies`

func (c *Chromium) extractCookies(masterKey []byte, path string) ([]types.CookieEntry, error) {
    cookies, err := sqliteutil.QueryRows(path, false, c.query(types.Cookie),
        func(rows *sql.Rows) (types.CookieEntry, error) {
            var (
                name, host, path                               string
                isSecure, isHTTPOnly, hasExpire, isPersistent   int
                createdAt, expireAt                             int64
                encryptedValue                                  []byte
            )
            if err := rows.Scan(&name, &encryptedValue, &host, &path,
                &createdAt, &expireAt, &isSecure, &isHTTPOnly,
                &hasExpire, &isPersistent); err != nil {
                return types.CookieEntry{}, err
            }

            value, _ := decryptValue(masterKey, encryptedValue)
            return types.CookieEntry{
                Name:       name,
                Host:       host,
                Path:       path,
                Value:      string(value),
                IsSecure:   isSecure != 0,
                IsHTTPOnly: isHTTPOnly != 0,
                ExpireAt:   typeutil.TimeEpoch(expireAt),
                CreatedAt:  typeutil.TimeEpoch(createdAt),
            }, nil
        })
    if err != nil { return nil, err }

    sort.Slice(cookies, func(i, j int) bool {
        return cookies[i].CreatedAt.After(cookies[j].CreatedAt)
    })
    return cookies, nil
}
```

### 4.3 Firefox password (JSON + `decryptPBE()` helper)

Firefox uses `decryptPBE()` to combine the 3-step pipeline (base64 decode -> ASN1 PBE parse -> decrypt) into one call, reducing 6 error checks to 2.

```go
// browser/firefox/extract_password.go

// decryptPBE combines base64 decode + ASN1 PBE parse + decrypt.
func decryptPBE(encoded string, masterKey []byte) ([]byte, error) {
    raw, err := base64.StdEncoding.DecodeString(encoded)
    if err != nil { return nil, fmt.Errorf("base64 decode: %w", err) }
    pbe, err := crypto.NewASN1PBE(raw)
    if err != nil { return nil, fmt.Errorf("parse asn1 pbe: %w", err) }
    plaintext, err := pbe.Decrypt(masterKey)
    if err != nil { return nil, fmt.Errorf("decrypt: %w", err) }
    return plaintext, nil
}

func (f *Firefox) extractPasswords(masterKey []byte, path string) ([]types.LoginEntry, error) {
    data, err := os.ReadFile(path)
    if err != nil { return nil, err }

    var logins []types.LoginEntry
    for _, v := range gjson.GetBytes(data, "logins").Array() {
        user, err := decryptPBE(v.Get("encryptedUsername").String(), masterKey)
        if err != nil {
            log.Debugf("decrypt username: %v", err)
            continue
        }
        pwd, err := decryptPBE(v.Get("encryptedPassword").String(), masterKey)
        if err != nil {
            log.Debugf("decrypt password: %v", err)
            continue
        }

        url := v.Get("formSubmitURL").String()
        if url == "" { url = v.Get("hostname").String() }

        logins = append(logins, types.LoginEntry{
            URL:       url,
            Username:  string(user),
            Password:  string(pwd),
            CreatedAt: typeutil.TimeStamp(v.Get("timeCreated").Int() / 1000),
        })
    }

    sort.Slice(logins, func(i, j int) bool {
        return logins[i].CreatedAt.After(logins[j].CreatedAt)
    })
    return logins, nil
}
```

### 4.4 Firefox cookie (SQLite, no encryption)

```go
// browser/firefox/extract_cookie.go

const firefoxCookieQuery = `SELECT name, value, host, path,
    creationTime, expiry, isSecure, isHttpOnly FROM moz_cookies`

func (f *Firefox) extractCookies(path string) ([]types.CookieEntry, error) {
    cookies, err := sqliteutil.QueryRows(path, true, firefoxCookieQuery,
        func(rows *sql.Rows) (types.CookieEntry, error) {
            var (
                name, value, host, path string
                isSecure, isHTTPOnly    int
                createdAt, expiry       int64
            )
            if err := rows.Scan(&name, &value, &host, &path,
                &createdAt, &expiry, &isSecure, &isHTTPOnly); err != nil {
                return types.CookieEntry{}, err
            }
            return types.CookieEntry{
                Name:       name,
                Host:       host,
                Path:       path,
                Value:      value,  // not encrypted
                IsSecure:   isSecure != 0,
                IsHTTPOnly: isHTTPOnly != 0,
                ExpireAt:   typeutil.TimeStamp(expiry),
                CreatedAt:  typeutil.TimeStamp(createdAt / 1000000),
            }, nil
        })
    if err != nil { return nil, err }

    sort.Slice(cookies, func(i, j int) bool {
        return cookies[i].CreatedAt.After(cookies[j].CreatedAt)
    })
    return cookies, nil
}
```

### 4.5 Chromium local storage (LevelDB)

```go
// browser/chromium/extract_storage.go

func (c *Chromium) extractLocalStorage(path string) ([]types.StorageEntry, error) {
    db, err := leveldb.OpenFile(path, nil)
    if err != nil { return nil, err }
    defer db.Close()

    var entries []types.StorageEntry
    iter := db.NewIterator(nil, nil)
    defer iter.Release()

    for iter.Next() {
        url, name := parseStorageKey(iter.Key(), []byte{0}) // \x00 separator
        if url == "" { continue }
        entries = append(entries, types.StorageEntry{
            URL:   url,
            Key:   name,
            Value: string(iter.Value()),
        })
    }
    return entries, iter.Error()
}

func (c *Chromium) extractSessionStorage(path string) ([]types.StorageEntry, error) {
    db, err := leveldb.OpenFile(path, nil)
    if err != nil { return nil, err }
    defer db.Close()

    var entries []types.StorageEntry
    iter := db.NewIterator(nil, nil)
    defer iter.Release()

    for iter.Next() {
        url, name := parseStorageKey(iter.Key(), []byte("-")) // "-" separator
        if url == "" { continue }
        entries = append(entries, types.StorageEntry{
            URL:   url,
            Key:   name,
            Value: string(iter.Value()),
        })
    }
    return entries, iter.Error()
}

func parseStorageKey(key []byte, separator []byte) (url, name string) {
    parts := bytes.SplitN(key, separator, 2)
    if len(parts) != 2 { return "", "" }
    return string(parts[0]), string(parts[1])
}
```

### 4.6 Key differences between engines

| Aspect | Chromium | Firefox |
|--------|----------|---------|
| Password source | SQLite (`Login Data`) | JSON (`logins.json`) |
| Password decryption | DPAPI → AES-GCM/CBC | ASN1PBE |
| Cookie encryption | Yes (masterKey needed) | No (plaintext) |
| Cookie journal_mode | Not needed | `PRAGMA journal_mode=off` |
| Time format | WebKit epoch (`TimeEpoch`) | Unix microseconds (`TimeStamp / 1e6`) |
| Storage format | LevelDB directory | SQLite (`webappsstore.sqlite`) |
| key4.db | Not used | Required for master key derivation |
| masterKey parameter | Passed to password, cookie, creditcard | Passed to password only |

### 4.7 Error handling in extract methods

Three-level rule:

| Level | Action | Example |
|-------|--------|---------|
| File/DB open failure | `return nil, err` | `os.ReadFile` fails, `sql.Open` fails |
| Single record failure | `log.Debugf` + `continue` | One password decryption failed |
| Entire Category failure | Collected into `errs` by caller | Cookie file locked |

Extract methods only `return error` for file-level failures. Record-level failures are logged at Debug level and skipped. The caller (`Extract()`) collects per-category errors with `errors.Join`.

Error wrapping uses `fmt.Errorf("context: %w", err)` — no custom error types.

---

## 5. File Acquisition Layer

### 5.1 Session manager

```go
// filemanager/session.go

type Session struct {
    tempDir string
}

func NewSession() (*Session, error) {
    dir, err := os.MkdirTemp("", "hbd-*")
    if err != nil { return nil, err }
    return &Session{tempDir: dir}, nil
}

func (s *Session) TempDir() string { return s.tempDir }

func (s *Session) Acquire(src, dst string, isDir bool) error {
    if isDir {
        return fileutil.CopyDir(src, dst, "lock")
    }
    // Try normal copy first
    err := fileutil.CopyFile(src, dst)
    if err != nil {
        // Normal copy failed (file may be locked), try platform-specific method
        if err2 := copyLocked(src, dst); err2 != nil {
            return fmt.Errorf("copy %s: %w; locked copy: %v", src, err, err2)
        }
    }
    // Copy SQLite WAL/SHM companion files if present
    for _, suffix := range []string{"-wal", "-shm"} {
        if fileutil.IsFileExists(src + suffix) {
            _ = fileutil.CopyFile(src+suffix, dst+suffix)
        }
    }
    return nil
}

func (s *Session) Cleanup() {
    os.RemoveAll(s.tempDir)
}
```

### 5.2 Locked file handling (Windows)

On Windows, Chrome locks Cookie files while running. `Session.Acquire()` falls back to `copyLocked()` which uses `syscall.CreateFile` with `FILE_SHARE_READ|FILE_SHARE_WRITE|FILE_SHARE_DELETE` flags to bypass exclusive locks.

Platform-specific files:
- `filemanager/copy_windows.go` — `copyLocked()` with sharing flags
- `filemanager/copy_other.go` — stub returning error

This is transparent to callers — browser extract methods never know whether a file was copied normally or via the locked-file path.

### 5.3 Acquirer interface (deferred)

If only `CopyAcquirer` is needed, `Session.Acquire()` handles it directly. The `Acquirer` interface can be introduced later when VSS or other strategies are needed.

---

## 6. Output

```go
// browserdata/output.go

func (d *BrowserData) Output(dir, browserName, format string) error {
    items := []struct {
        name string
        data interface{}
        len  int
    }{
        {"password", d.Passwords, len(d.Passwords)},
        {"cookie", d.Cookies, len(d.Cookies)},
        {"bookmark", d.Bookmarks, len(d.Bookmarks)},
        {"history", d.Histories, len(d.Histories)},
        {"download", d.Downloads, len(d.Downloads)},
        {"creditcard", d.CreditCards, len(d.CreditCards)},
        {"extension", d.Extensions, len(d.Extensions)},
        {"localstorage", d.LocalStorage, len(d.LocalStorage)},
        {"sessionstorage", d.SessionStorage, len(d.SessionStorage)},
    }

    var errs []error
    for _, item := range items {
        if item.len == 0 { continue }
        filename := formatFilename(browserName, item.name, format)
        if err := writeFile(dir, filename, format, item.data); err != nil {
            errs = append(errs, fmt.Errorf("write %s: %w", filename, err))
            continue
        }
        log.Infof("exported: %s (%d items)", filename, item.len)
    }
    return errors.Join(errs...)
}

func writeFile(dir, filename, format string, data interface{}) error {
    if dir != "" {
        if err := os.MkdirAll(dir, 0o750); err != nil { return err }
    }
    path := filepath.Join(dir, filename)
    f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
    if err != nil { return err }
    defer f.Close()

    switch format {
    case "json":
        return writeJSON(f, data)
    default:
        return writeCSV(f, data)
    }
}

func writeJSON(w io.Writer, data interface{}) error {
    enc := json.NewEncoder(w)
    enc.SetIndent("", "  ")
    enc.SetEscapeHTML(false)
    return enc.Encode(data)
}

func writeCSV(w io.Writer, data interface{}) error {
    // UTF-8 BOM (3 bytes) — replaces golang.org/x/text dependency
    w.Write([]byte{0xEF, 0xBB, 0xBF})
    csvWriter := csv.NewWriter(w)
    return gocsv.MarshalCSV(data, gocsv.NewSafeCSVWriter(csvWriter))
}

func formatFilename(browserName, dataName, format string) string {
    r := strings.NewReplacer(" ", "_", ".", "_", "-", "_")
    ext := "csv"
    if format == "json" { ext = "json" }
    return strings.ToLower(fmt.Sprintf("%s_%s.%s", r.Replace(browserName), dataName, ext))
}
```

---

## 7. What Was Eliminated

| Before | After | Why |
|--------|-------|-----|
| `extractor/` package (interface + registry + factory) | Deleted | Browser engines have typed extract methods |
| `browserdata/password/`, `cookie/`, etc. (9 sub-packages) | Deleted | Extract logic moved into `browser/chromium/` and `browser/firefox/` |
| `browserdata/imports.go` | Deleted | No init() registration needed |
| `types.DataType` (22 iota constants) | `types.Category` (9 constants) | No browser prefix, no key types |
| `itemFileNames` map | `chromiumSources` / `firefoxSources` per engine | File layout is engine-internal |
| `TempFilename()` on DataType | `Session.TempDir()` + `filepath.Base()` | Session manages temp paths |
| `DefaultChromiumTypes`, `DefaultFirefoxTypes`, `DefaultYandexTypes` | `types.AllCategories` | One list for all engines |
| `loginData.encryptPass`, `cookie.encryptValue` | Local variables in extract methods | Encrypted fields don't belong in data models |
| 20 trivial `Name()` / `Len()` methods | Not needed | No Extractor interface |

---

## 8. Implementation Plan

### Phase 1: Foundation (new files only, zero risk)

1. `types/category.go` — Category enum
2. `types/models.go` — all *Entry structs
3. `browserdata/browserdata.go` — BrowserData struct
4. `utils/sqliteutil/sqlite.go` — QuerySQLite()
5. `browser/chromium/decrypt.go` — decryptValue() (Chromium-specific, unexported)
6. `filemanager/session.go` — Session

### Phase 2: Extract methods (new files, coexist with old code)

1. `browser/chromium/source.go` — chromiumSources, yandexSources
2. `browser/chromium/extract_*.go` — all 9 extract methods
3. `browser/firefox/source.go` — firefoxSources
4. `browser/firefox/extract_*.go` — all extract methods

### Phase 3: Wiring (modify existing files)

1. Update `Chromium.Extract()` to use new extract methods
2. Update `Firefox.Extract()` to use new extract methods
3. Update `Config` and `PickBrowsers()`
4. Update `browserdata/output.go`
5. Update CLI `main.go`

### Phase 4: Cleanup (delete old code)

1. Delete `extractor/` package
2. Delete `browserdata/imports.go`
3. Delete `browserdata/password/`, `cookie/`, etc.
4. Delete old `types.DataType`, `itemFileNames`
5. Delete `browser/consts.go`

### Phase 5: Verification

```bash
go test ./...
go vet ./...
gofmt -d .
GOOS=windows GOARCH=amd64 go build ./cmd/hack-browser-data/
GOOS=linux GOARCH=amd64 go build ./cmd/hack-browser-data/
GOOS=darwin GOARCH=amd64 go build ./cmd/hack-browser-data/
```

---

## 9. Open Questions

1. **Sort direction**: standardize all categories to DESC by date?
2. **Output format**: keep `gocsv` or switch to `encoding/csv`?
3. **LevelDB key parsing**: the current `fillKey`/`fillHeader`/`fillValue` logic in localstorage is complex — how much of that detail carries over?

---

## 10. Relationship with RFC-001

| Area | RFC-001 | RFC-002 (this doc) |
|------|---------|-------------------|
| Data model (Category + *Entry) | defines | uses |
| BrowserData container | defines | implements Output |
| Cipher version | covered | — |
| Master key retrieval | covered | — |
| Browser registration | covered | — |
| Yandex variant | covered | — |
| Error handling pattern | covered | — |
| File source mapping | — | covered |
| File acquisition | — | covered |
| Extract methods | — | covered |
| sqliteutil helpers | — | covered |
| Output | — | covered |
