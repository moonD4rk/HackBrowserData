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

### File organization

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

Example `password.json`:
```json
[
  {"browser":"Chrome","profile":"Default","url":"https://example.com","username":"alice","password":"xxx","created_at":"2026-01-01T00:00:00Z"},
  {"browser":"Firefox","profile":"abc123.default","url":"https://reddit.com","username":"charlie","password":"zzz","created_at":"2026-03-01T00:00:00Z"}
]
```

### Architecture: encapsulated Writer struct

The `Writer` struct is the only exported type. All internals (formatter,
row types, file management) are unexported. Caller sees 3 methods only.

```go
// output/output.go — the only exported type

type Writer struct {
    dir       string
    formatter formatter      // unexported
    results   []result       // unexported
}

func NewWriter(dir, format string) (*Writer, error) {
    f, err := newFormatter(format)
    if err != nil {
        return nil, err
    }
    return &Writer{dir: dir, formatter: f}, nil
}

func (w *Writer) Add(browser, profile string, data *types.BrowserData) {
    w.results = append(w.results, result{browser, profile, data})
}

func (w *Writer) Write() error {
    // 1. aggregate all results by category into row slices
    // 2. for each non-empty category, format to buffer, write file
}
```

Caller code (3 lines):

```go
w, _ := output.NewWriter(dir, "csv")
for _, b := range browsers {
    data, _ := b.Extract(categories)
    w.Add(b.BrowserName(), b.ProfileName(), data)
}
w.Write()
```

### Data layer stays pure

Entry structs do NOT contain browser/profile. Each field carries both
`json` and `csv` struct tags — JSON output reads `json` tags, CSV output
reads `csv` tags via reflection. No methods on entry types.

```go
// types/models.go — pure data, no methods
type LoginEntry struct {
    URL       string    `json:"url" csv:"url"`
    Username  string    `json:"username" csv:"username"`
    Password  string    `json:"password" csv:"password"`
    CreatedAt time.Time `json:"created_at" csv:"created_at"`
}
```

### Internal row type (unexported)

A single `row` type wraps any entry with browser/profile context:

```go
// output/row.go — unexported

type row struct {
    Browser string
    Profile string
    entry   any
}
```

- **CSV**: `row.csvHeader()` / `row.csvRow()` use reflection to read `csv`
  struct tags and convert field values to strings (handles string, bool,
  int, int64, time.Time).
- **JSON**: `row.MarshalJSON()` uses `reflect.StructOf` to dynamically
  build a flat struct with browser/profile fields followed by entry fields,
  then delegates to `json.Marshal`. No manual string concatenation.

### Internal formatter interface (unexported)

```go
// output/formatter.go — unexported

type formatter interface {
    format(w io.Writer, rows []row) error
    ext() string
}

func newFormatter(name string) (formatter, error) {
    switch name {
    case "csv":           return &csvFormatter{}, nil
    case "json":          return &jsonFormatter{}, nil
    case "cookie-editor": return &cookieEditorFormatter{}, nil
    default:              return nil, fmt.Errorf("unsupported format: %s", name)
    }
}
```

### Format support

**CSV** (default):
- Standard `encoding/csv` — **no gocsv dependency**
- UTF-8 BOM for Excel compatibility
- Headers and values derived from `csv` struct tags via reflection

**JSON**:
- Valid JSON Array per file (not JSON Lines)
- Pretty-printed with `json.Encoder`, no HTML escape
- `reflect.StructOf` dynamically flattens browser/profile + entry fields

**CookieEditor** (`--format cookie-editor`):
- Only exports cookies, other categories skipped
- Field mapping: host→domain, IsSecure→secure, ExpireAt→expirationDate (unix)

### Dependency changes

- **Remove**: `github.com/gocarina/gocsv`
- **Remove**: `golang.org/x/text` (UTF-8 BOM = 3 bytes directly)
- **Add**: `github.com/spf13/cobra`

### Output package structure

```
output/
├── output.go           # Writer struct (exported): NewWriter(), Add(), Write()
├── row.go              # Unified row type (unexported) + MarshalJSON
├── reflect.go          # Reflection helpers: csv tag parsing, field formatting
├── formatter.go        # formatter interface (unexported) + newFormatter()
├── csv.go              # csvFormatter (unexported)
├── json.go             # jsonFormatter (unexported)
└── cookie_editor.go    # cookieEditorFormatter (unexported)
```

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
  → w, _ := output.NewWriter(dir, format)
  → For each browser:
      → data, _ := b.Extract(categories)
      → w.Add(b.BrowserName(), b.ProfileName(), data)
  → w.Write()
  → Optional: compress dir to zip

CLI (cobra list)
  → browser.Pick("all", "")  → []Browser
  → For each browser:
      → Print BrowserName() + ProfileName() + profileDir
      → If --detail: Extract + count entries
```

## 5. Implementation status

- [x] `output/` package: Writer struct + unified row type + reflection-based CSV/JSON + formatters
- [x] `types/category.go`: removed Each() and CategoryData
- [x] `types/models.go`: pure data structs with `json` + `csv` tags, no methods
- [x] Tests: 27 tests covering CSV/JSON/CookieEditor output, reflection helpers, MarshalJSON, csv tag coverage
- [ ] (PR 2) Rewrite browser dispatch + cobra CLI
- [ ] (PR 3) Delete old code + rename files

## 6. Future extensions

- `--group-by browser` — one file per browser+category (group by browser)
- `--group-by profile` — one file per browser+profile+category (group by profile)
- `--format netscape` — Netscape cookie.txt format (curl/wget compatible)
- `--format har` — HAR (HTTP Archive) format
