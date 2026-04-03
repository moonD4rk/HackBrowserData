# RFC-004: CLI (Cobra) and Output Design

**Author**: moonD4rk
**Status**: Proposed
**Created**: 2026-04-03
**Updated**: 2026-04-03

## Context

v2 architecture delivers `Extract() → *types.BrowserData`. The remaining
pieces are: CLI for user interaction and output for writing results to files.
Current CLI uses `urfave/cli` with flat flags; migrating to `cobra` with
subcommands for better extensibility.

## 1. CLI Design

### Subcommands

```
hack-browser-data
├── dump              # extract browser data (default when no subcommand)
│   ├── -b, --browser      all|chrome|firefox|...  (default: all)
│   ├── -c, --category     all|password,cookie,...  (default: all)
│   ├── -f, --format       csv|json|cookie-editor   (default: csv)
│   ├── -d, --dir          output directory          (default: results)
│   ├── -p, --profile-path custom profile path
│   ├──     --keychain-pw  macOS keychain password
│   └──     --zip          compress output
│
├── list              # show detected browsers and profile paths
│   └──     --detail  show per-category entry counts (no decryption)
│
└── global flags
    ├── -v, --verbose
    └──     --version
```

Running `hack-browser-data` with no subcommand defaults to `dump`.

### Examples

```bash
hack-browser-data                                          # dump all
hack-browser-data dump -b chrome -c password,cookie        # specific
hack-browser-data dump -b chrome -f json                   # JSON output
hack-browser-data dump -f cookie-editor                    # CookieEditor format
hack-browser-data list                                     # show browsers
hack-browser-data list --detail                            # show counts
```

### Removed/changed flags vs current CLI

| Current flag | Action | Reason |
|-------------|--------|--------|
| `--full-export` | Removed | Replaced by `--category all` (default) |
| `--results-dir` | Renamed `--dir` | Shorter |
| — | New `--category` | Fine-grained control |
| — | New `--keychain-pw` | macOS keychain password |
| — | New `--format cookie-editor` | CookieEditor compatibility |

### Code structure

```
cmd/hack-browser-data/
├── main.go       # cobra root command setup
├── dump.go       # dump subcommand
└── list.go       # list subcommand
```

## 2. Output Design

### File organization (方案 B)

One file per category. Browser and profile are columns, not filenames:

```
results/
├── password.csv
├── cookie.csv
├── history.csv
├── bookmark.csv
├── download.csv
├── extension.csv
├── creditcard.csv
├── localstorage.csv
└── sessionstorage.csv
```

At most 9 files, regardless of how many browsers/profiles.

Example `password.csv`:
```
browser,profile,url,username,password,created_at
Chrome,Default,https://example.com,alice,xxx,2026-01-01
Chrome,Profile 1,https://github.com,bob,yyy,2026-02-01
Firefox,abc123.default,https://reddit.com,charlie,zzz,2026-03-01
```

### Data layer stays pure

Entry structs do NOT contain browser/profile — that's output context:

```go
// types/models.go — unchanged
type LoginEntry struct {
    URL       string
    Username  string
    Password  string
    CreatedAt time.Time
}
```

Output layer wraps with context:

```go
// output layer adds browser/profile as prefix columns
w.Write(append([]string{"browser", "profile"}, entry.CSVHeader()...))
w.Write(append([]string{browserName, profileName}, entry.CSVRow()...))

// JSON uses embedding to flatten
type jsonRow struct {
    Browser string `json:"browser"`
    Profile string `json:"profile"`
    LoginEntry          // fields auto-expand
}
```

### Format support

**CSV** (default):
- Standard `encoding/csv` — **no gocsv dependency**
- UTF-8 BOM for Excel compatibility
- Each Entry type implements CSVRecord interface:

```go
type CSVRecord interface {
    CSVHeader() []string
    CSVRow() []string
}
```

**JSON**:
- `encoding/json` with `SetIndent`, no HTML escape
- Wrapped with browser/profile fields via struct embedding

**CookieEditor** (`--format cookie-editor`):
- Only exports cookies, other categories skipped
- Field mapping to CookieEditor's expected format:

```go
type cookieEditorEntry struct {
    Domain         string  `json:"domain"`         // ← Host
    ExpirationDate float64 `json:"expirationDate"` // ← ExpireAt.Unix()
    HTTPOnly       bool    `json:"httpOnly"`        // ← IsHTTPOnly
    Name           string  `json:"name"`
    Path           string  `json:"path"`
    Secure         bool    `json:"secure"`          // ← IsSecure
    Value          string  `json:"value"`
}
```

### Dependency changes

- **Remove**: `github.com/gocarina/gocsv`
- **Remove**: `golang.org/x/text` (UTF-8 BOM can be 3 bytes written directly)
- **Add**: `github.com/spf13/cobra`

## 3. `list` Command

### Basic mode

Shows real filesystem paths detected by `NewBrowsers`. No database access.

```
$ hack-browser-data list

Browser    Profile                   Path
Chrome     Default                   /Users/x/Library/.../Google/Chrome/Default
Chrome     Profile 1                 /Users/x/Library/.../Google/Chrome/Profile 1
Firefox    abc123.default-release    /Users/x/Library/.../Firefox/Profiles/abc123...
```

### Detail mode (`--detail`)

Counts entries per category without decryption:

```
$ hack-browser-data list --detail

Browser    Profile                 Password  Cookie  History  Bookmark  Extension
Chrome     Default                       1    3544       66       852         39
Chrome     Profile 1                     2     802       32         0          3
Firefox    abc123.default-release        3      48       53         7          0
```

## 4. Data flow

```
CLI (cobra dump)
  → Parse flags: browser, category, format, dir, keychain-pw
  → browser.Pick(browserName, keychainPwd)  → []Browser
  → For each browser:
      → b.Extract(categories) → *types.BrowserData
      → output.Write(data, b.Name(), profileName, dir, format)
         → data.Each(): iterate non-empty categories
         → For each category: append rows to category file
            (browser + profile as prefix columns)
  → Optional: compress dir to zip

CLI (cobra list)
  → browser.Pick("all", "")  → []Browser
  → For each browser:
      → Print Name() + profileDir
      → If --detail: Extract + count entries
```

## 5. Implementation order

1. Entry types implement `CSVRecord` interface
2. Add `BrowserData.Each()` to `types/`
3. Create output logic (CSV/JSON/CookieEditor writer)
4. Rewrite `browser/browser.go` dispatch with v2 API
5. Rewrite platform browser lists (`browser_darwin.go`, etc.)
6. Create cobra CLI (`cmd/hack-browser-data/`)
7. Remove `gocsv` + `golang.org/x/text` dependencies
8. Delete old code (`browserdata/`, `extractor/`, old `chromium.go`/`firefox.go`)
9. Rename `chromium_new.go` → `chromium.go`, `firefox_new.go` → `firefox.go`
10. Update RFCs to reflect final state

## 6. Future extensions

- `--group-by browser` — one file per browser+category (方案 C)
- `--group-by profile` — one file per browser+profile+category (方案 A)
- `--format netscape` — Netscape cookie.txt format (curl/wget compatible)
- `--format har` — HAR (HTTP Archive) format
