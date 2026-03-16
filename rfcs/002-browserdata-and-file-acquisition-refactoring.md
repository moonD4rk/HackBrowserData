# RFC-002: Data Extraction & File Acquisition

**Author**: moonD4rk
**Status**: Proposed
**Created**: 2026-03-14
**Updated**: 2026-03-16

## Abstract

This RFC covers the implementation details of data extraction and file acquisition:

1. **File source mapping**: how each browser engine maps categories to files
2. **File acquisition**: Session-based temp file management with deduplication
3. **Extract methods**: concrete implementations for each data category
4. **Shared helpers**: `QuerySQLite()` and `DecryptChromiumValue()`
5. **Output**: writing `BrowserData` to CSV/JSON files

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
    │  platformBrowsers() → []BrowserConfig
    │  → chromium.New(cfg, dir) / firefox.New(dir)
    ▼
Browser.BrowsingData(categories)
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

```go
func (f *Firefox) BrowsingData(categories []types.Category) (*browserdata.BrowserData, error) {
    session, _ := filemanager.NewSession()
    defer session.Cleanup()

    files := f.acquireFiles(session, categories)

    // key4.db is infrastructure — acquired separately
    keyPath := filepath.Join(session.TempDir(), "key4.db")
    session.Acquire(filepath.Join(f.profileDir, "key4.db"), keyPath, false)
    masterKey, err := f.deriveMasterKey(keyPath)
    if err != nil { return nil, err }

    // ... extract each category (see Section 4)
}
```

---

## 3. Shared Helpers: `browserdata/datautil/`

### 3.1 SQLite query helper

```go
// browserdata/datautil/sqlite.go

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
package datautil

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

```go
// browserdata/datautil/decrypt.go

func DecryptChromiumValue(masterKey, encrypted []byte) ([]byte, error) {
    if len(encrypted) == 0 { return nil, nil }
    if len(masterKey) == 0 {
        return crypto.DecryptWithDPAPI(encrypted)
    }
    value, err := crypto.DecryptWithDPAPI(encrypted)
    if err != nil {
        value, err = crypto.DecryptWithChromium(masterKey, encrypted)
    }
    return value, err
}
```

---

## 4. Extract Method Examples

Each extract method lives in its own `extract_*.go` file inside the browser engine package (see RFC-001 for naming convention). The default SQL query is a `const` in the same file. Override is checked via `c.queryOverrides`.

### 4.1 Chromium password (SQLite + decryption)

```go
// browser/chromium/extract_password.go

const defaultLoginQuery = `SELECT origin_url, username_value, password_value, date_created FROM logins`

func (c *Chromium) extractPasswords(masterKey []byte, path string) ([]types.LoginEntry, error) {
    logins, err := datautil.QueryRows(path, false, c.query(types.Password),
        func(rows *sql.Rows) (types.LoginEntry, error) {
            var url, username string
            var pwd []byte
            var created int64
            if err := rows.Scan(&url, &username, &pwd, &created); err != nil {
                return types.LoginEntry{}, err
            }
            password, _ := datautil.DecryptChromiumValue(masterKey, pwd)
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
    cookies, err := datautil.QueryRows(path, false, c.query(types.Cookie),
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

            value, _ := datautil.DecryptChromiumValue(masterKey, encryptedValue)
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

### 4.3 Firefox password (JSON + ASN1PBE decryption)

```go
// browser/firefox/extract_password.go

func (f *Firefox) extractPasswords(masterKey []byte, path string) ([]types.LoginEntry, error) {
    data, err := os.ReadFile(path)
    if err != nil { return nil, err }

    var logins []types.LoginEntry
    loginsJSON := gjson.GetBytes(data, "logins")
    for _, v := range loginsJSON.Array() {
        encUser, _ := base64.StdEncoding.DecodeString(v.Get("encryptedUsername").String())
        encPass, _ := base64.StdEncoding.DecodeString(v.Get("encryptedPassword").String())

        userPBE, _ := crypto.NewASN1PBE(encUser)
        pwdPBE, _ := crypto.NewASN1PBE(encPass)
        user, _ := userPBE.Decrypt(masterKey)
        pwd, _ := pwdPBE.Decrypt(masterKey)

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
    cookies, err := datautil.QueryRows(path, true, firefoxCookieQuery,
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
    if err := fileutil.CopyFile(src, dst); err != nil { return err }
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

### 5.2 Acquirer interface (deferred)

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

    out := newOutPutter(format)
    var errs []error
    for _, item := range items {
        if item.len == 0 { continue }

        filename := fileutil.Filename(browserName, item.name, out.Ext())
        f, err := out.CreateFile(dir, filename)
        if err != nil {
            errs = append(errs, fmt.Errorf("create %s: %w", filename, err))
            continue
        }
        if err := out.Write(item.data, f); err != nil {
            errs = append(errs, fmt.Errorf("write %s: %w", filename, err))
        }
        f.Close()
        log.Infof("exported: %s (%d items)", filename, item.len)
    }
    return errors.Join(errs...)
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
4. `browserdata/datautil/sqlite.go` — QuerySQLite()
5. `browserdata/datautil/decrypt.go` — DecryptChromiumValue()
6. `filemanager/session.go` — Session

### Phase 2: Extract methods (new files, coexist with old code)

1. `browser/chromium/source.go` — chromiumSources, yandexSources
2. `browser/chromium/extract_*.go` — all 9 extract methods
3. `browser/firefox/source.go` — firefoxSources
4. `browser/firefox/extract_*.go` — all extract methods

### Phase 3: Wiring (modify existing files)

1. Update `Chromium.BrowsingData()` to use new extract methods
2. Update `Firefox.BrowsingData()` to use new extract methods
3. Update `BrowserConfig` and `PickBrowsers()`
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
| datautil helpers | — | covered |
| Output | — | covered |
