# RFC-002: Data Extraction & File Acquisition Refactoring

**Author**: moonD4rk
**Status**: Proposed
**Created**: 2026-03-14
**Updated**: 2026-03-15

## Abstract

This RFC focuses on two subsystems of HackBrowserData:

1. **Data extraction**: how browser engines extract and decrypt data into `types.LoginEntry`, `types.CookieEntry`, etc.
2. **File acquisition**: how browser files are discovered, copied, and cleaned up

**Constraint**: Go 1.20 (Windows 7 support).

See RFC-001 for the overall architecture, data model redesign, crypto layer, browser registration, and CLI separation.

---

## 1. Data Flow Overview

```
Browser.BrowsingData(categories)
    │
    ├─ filemanager.Session
    │   └─ acquireFiles(sources, categories) → map[Category]tempPath
    │
    ├─ masterKey (Chromium: KeyRetriever / Firefox: key4.db derivation)
    │
    └─ per-category extract methods
        │
        │  Chromium: c.extractPasswords(masterKey, tempPath) → []types.LoginEntry
        │  Firefox:  f.extractPasswords(masterKey, tempPath) → []types.LoginEntry
        │                                                        ↑ same output type
        │
        └─ browserdata.BrowserData{Passwords: [...], Cookies: [...], ...}
```

Each browser engine (Chromium/Firefox) has its own extract methods that know:
- Which SQL query or JSON structure to use
- How to decrypt (AES-GCM, AES-CBC, ASN1-PBE, or no decryption)
- How to map raw fields to `types.*Entry` models

The output is always the browser-agnostic models defined in `types/models.go`.

---

## 2. File Source Mapping

### 2.1 Design: Category → source (one flat map per engine)

Each browser engine defines a simple map from `types.Category` to file path candidates:

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

### 2.2 File acquisition with deduplication

When multiple categories map to the same file (e.g. History and Download both use "History"), the file is copied only once:

```go
func (c *Chromium) acquireFiles(session *filemanager.Session, categories []types.Category) map[types.Category]string {
    result := make(map[types.Category]string)
    copied := make(map[string]string) // abs src path → temp dst path

    for _, cat := range categories {
        src, ok := chromiumSources[cat]
        if !ok { continue }

        for _, rel := range src.paths {
            abs := filepath.Join(c.profileDir, rel)

            // already copied this file? reuse
            if dst, ok := copied[abs]; ok {
                result[cat] = dst
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

Firefox's `key4.db` is required for master key derivation but is NOT a data category. It is acquired separately:

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

    data := &browserdata.BrowserData{}
    for _, cat := range categories {
        // ... extract each category
    }
    return data, nil
}
```

---

## 3. Shared Extract Helpers: `browserdata/datautil/`

### 3.1 SQLite query helper — `datautil/sqlite.go`

```go
package datautil

import (
    "database/sql"
    _ "modernc.org/sqlite"
    "github.com/moond4rk/hackbrowserdata/log"
)

// QuerySQLite opens a SQLite DB, optionally disables journal mode,
// runs the query, and calls scanFn for each row.
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
            continue
        }
    }
    return rows.Err()
}
```

### 3.2 Chromium decrypt helper — `datautil/decrypt.go`

```go
package datautil

import "github.com/moond4rk/hackbrowserdata/crypto"

// DecryptChromiumValue tries DPAPI first, then Chromium AES-GCM/CBC.
// If masterKey is empty, only tries DPAPI (Yandex behavior).
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

### 4.1 Chromium password extraction

```go
// browser/chromium/extract_password.go

const queryChromiumLogin = `SELECT origin_url, username_value, password_value, date_created FROM logins`

func (c *Chromium) extractPasswords(masterKey []byte, dbPath string) ([]types.LoginEntry, error) {
    var logins []types.LoginEntry
    err := datautil.QuerySQLite(dbPath, false, queryChromiumLogin, func(rows *sql.Rows) error {
        var url, username string
        var pwd []byte
        var created int64
        if err := rows.Scan(&url, &username, &pwd, &created); err != nil { return err }

        password, _ := datautil.DecryptChromiumValue(masterKey, pwd)
        logins = append(logins, types.LoginEntry{
            URL:       url,
            Username:  username,
            Password:  string(password),
            CreatedAt: typeutil.TimeEpoch(created),
        })
        return nil
    })
    if err != nil { return nil, err }

    sort.Slice(logins, func(i, j int) bool {
        return logins[i].CreatedAt.After(logins[j].CreatedAt)
    })
    return logins, nil
}
```

### 4.2 Firefox password extraction

```go
// browser/firefox/extract_password.go

func (f *Firefox) extractPasswords(masterKey []byte, jsonPath string) ([]types.LoginEntry, error) {
    data, err := os.ReadFile(jsonPath)
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

### 4.3 Key point

Both `extractPasswords` methods return `[]types.LoginEntry`. The caller (BrowsingData) doesn't know or care which engine produced it. Encrypted bytes (`encryptedPwd`, `encUser`) are local variables inside extract methods — they never leak into the data models.

---

## 5. File Acquisition Layer

### 5.1 Session manager — `filemanager/session.go`

```go
package filemanager

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
    // Copy WAL/SHM companion files
    for _, suffix := range []string{"-wal", "-shm"} {
        walSrc := src + suffix
        if fileutil.IsFileExists(walSrc) {
            _ = fileutil.CopyFile(walSrc, dst+suffix)
        }
    }
    return nil
}

func (s *Session) Cleanup() {
    os.RemoveAll(s.tempDir)
}
```

### 5.2 Acquirer interface (optional, for future extensibility)

```go
// filemanager/acquirer.go

type Acquirer interface {
    Acquire(src, dst string, isDir bool) error
}

type CopyAcquirer struct{}

func (a *CopyAcquirer) Acquire(src, dst string, isDir bool) error {
    // same logic as Session.Acquire
}
```

If only `CopyAcquirer` is needed now, the `Acquirer` interface can be deferred. `Session.Acquire()` handles it directly.

---

## 6. Output

```go
// browserdata/output.go

func (d *BrowserData) Output(dir, browserName, format string) error {
    outputs := []struct {
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
    for _, o := range outputs {
        if o.len == 0 { continue }
        filename := fileutil.Filename(browserName, o.name, out.Ext())
        f, err := out.CreateFile(dir, filename)
        if err != nil { continue }
        out.Write(o.data, f)
        f.Close()
    }
    return nil
}
```

---

## 7. What Was Eliminated

### Extractor interface and registry (entire `extractor/` package)

**Before**: 20 types implementing `Extractor` interface, registered via `init()` in 9 packages, triggered by blank imports in `imports.go`.

**After**: Each browser engine has typed extract methods (`extractPasswords`, `extractCookies`, ...) that return `[]types.*Entry` directly. No interface, no registry, no factory.

### browserdata sub-packages (9 → 1)

**Before**: `password/`, `cookie/`, `bookmark/`, `history/`, `download/`, `creditcard/`, `extension/`, `localstorage/`, `sessionstorage/` — each with its own types and boilerplate.

**After**: Extract methods live inside `browser/chromium/` and `browser/firefox/`. `browserdata/` only contains `BrowserData` struct and output logic.

### DataType enum (22 → 0)

Replaced by `types.Category` (9 values) defined in RFC-001. No file mappings, no `TempFilename()`, no browser prefixes.

---

## 8. Implementation Plan

### Phase 1: Types and models (RFC-001)

1. Create `types/category.go` with `Category` enum
2. Create `types/models.go` with all `*Entry` structs
3. Create `browserdata/browserdata.go` with `BrowserData` struct

### Phase 2: Shared helpers

1. Create `browserdata/datautil/sqlite.go` with `QuerySQLite()`
2. Create `browserdata/datautil/decrypt.go` with `DecryptChromiumValue()`

### Phase 3: File acquisition

1. Create `filemanager/session.go`
2. Create `browser/chromium/source.go` with `chromiumSources`
3. Create `browser/firefox/source.go` with `firefoxSources`
4. Implement `acquireFiles()` with deduplication

### Phase 4: Extract methods

1. Implement Chromium extract methods (password, cookie, history, ...) in `browser/chromium/`
2. Implement Firefox extract methods in `browser/firefox/`
3. Wire into `BrowsingData()` with switch on Category
4. Update output logic in `browserdata/`

### Phase 5: Cleanup

1. Delete `extractor/` package
2. Delete `browserdata/imports.go`
3. Delete `browserdata/password/`, `cookie/`, etc. (9 sub-packages)
4. Delete old `types.DataType` and `itemFileNames`
5. Run tests, cross-platform build verification

---

## 9. Open Questions

1. **Yandex**: separate extract methods in `chromium/` (e.g. `extractYandexPasswords`) or unify with Chromium extract?
2. **Output format**: keep `gocsv` or switch to `encoding/csv` with manual header/row writing?
3. **LocalStorage/SessionStorage**: keep as extract methods or extract into shared helper since Chromium LevelDB logic is complex?
4. **Sort direction**: standardize all categories to sort DESC by date?

---

## 10. Relationship with RFC-001

| Area | RFC-001 | RFC-002 (this doc) |
|------|---------|-------------------|
| Data model redesign | defines `Category` + `*Entry` types | uses them |
| `BrowserData` container | defines struct | implements Output |
| Cipher version | covered | — |
| Master key retrieval | covered | — |
| Browser registration | covered | — |
| CLI separation | covered | — |
| Error types | covered | uses them |
| File source mapping | — | covered |
| File acquisition | — | covered |
| Extract methods | — | covered |
| datautil helpers | — | covered |
