# RFC-007: CLI & Output Design

**Author**: moonD4rk
**Status**: Living Document
**Created**: 2026-04-05

## 1. Command Structure

The CLI is built on [cobra](https://github.com/spf13/cobra) with three subcommands: `dump`, `list`, and `version`.

### 1.1 Root Command

The root command defines one persistent flag: `--verbose` / `-v` (enable debug logging).

**Default-to-dump**: when no subcommand is given, the root delegates to `dump`. All of `dump`'s flags are copied onto the root command, so `hack-browser-data -b chrome` and `hack-browser-data dump -b chrome` are equivalent.

### 1.2 dump Command

The primary command. Extracts, decrypts, and writes browser data to files.

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--browser` | `-b` | `"all"` | Target browser |
| `--category` | `-c` | `"all"` | Data categories (comma-separated) |
| `--format` | `-f` | `"csv"` | Output format: csv, json, cookie-editor |
| `--dir` | `-d` | `"results"` | Output directory |
| `--profile-path` | `-p` | | Custom profile directory |
| `--keychain-pw` | | | macOS keychain password |
| `--zip` | | `false` | Compress output to zip |

**Workflow**: PickBrowsers (filter by `-b`) → parseCategories (split `-c` on commas) → NewWriter (select formatter by `-f`) → Extract loop (each browser) → Write → optional CompressDir.

The nine recognized categories are: `password`, `cookie`, `bookmark`, `history`, `download`, `creditcard`, `extension`, `localstorage`, `sessionstorage`. The string `"all"` maps to all nine.

### 1.3 list Command

Lists all detected browsers and profiles via `text/tabwriter`.

**Basic mode** (default) — three columns: Browser, Profile, Path.

**Detail mode** (`--detail`) — adds a column for every category showing entry counts. This actually calls `Extract()` on each browser to count entries.

### 1.4 version Command

Prints version, commit hash (truncated to 8 chars), and build date. Values are injected at build time via `-ldflags`. When building without ldflags (development mode), falls back to `runtime/debug.ReadBuildInfo()` to extract `vcs.revision` and `vcs.time`.

## 2. Output Architecture

All output logic lives in the `output` package. Only one type is exported: `Writer`.

### 2.1 Writer

Three methods define the entire API:

- **`NewWriter(dir, format)`** — creates a writer with the specified formatter
- **`Add(browser, profile, data)`** — accumulates one browser profile's extraction results
- **`Write()`** — aggregates all results by category and writes each non-empty category to its own file

### 2.2 Row Type

An unexported `row` wraps any entry struct with browser and profile context. It provides CSV header/value generation via reflection on `csv` struct tags, and flat JSON output via `reflect.StructOf` dynamic struct building (browser + profile fields prepended to entry fields).

### 2.3 Formatter Interface

An unexported interface with two methods: `format(w, rows)` and `ext()` (file extension).

| Format | Extension | Description |
|--------|-----------|-------------|
| `csv` | `.csv` | Standard `encoding/csv`, reflection-based headers from `csv` struct tags |
| `json` | `.json` | `json.Encoder` with indent, no HTML escape, flat objects |
| `cookie-editor` | `.json` | CookieEditor-compatible format, non-cookie categories fall back to standard JSON |

## 3. Output Formats

### 3.1 CSV

Headers and values are extracted via reflection on the `csv` struct tag of each entry field. A UTF-8 BOM (`0xEF 0xBB 0xBF`) is prepended for Excel compatibility. Field types are converted to strings: `time.Time` → RFC3339, `bool` → `"true"`/`"false"`, integers → base-10 string.

### 3.2 JSON

Each row is serialized via `MarshalJSON()`, which uses `reflect.StructOf` to dynamically build a flat struct at runtime — browser and profile are top-level fields alongside the entry fields, avoiding nested JSON. The encoder uses two-space indent and disables HTML escaping to preserve URLs.

### 3.3 CookieEditor

Produces JSON compatible with the [CookieEditor](https://cookie-editor.cgagnier.ca/) browser extension. Cookie entries are converted to a specific field mapping:

| CookieEntry field | CookieEditor field | Notes |
|-------------------|--------------------|-------|
| Host | domain | |
| Path | path | |
| Name | name | |
| Value | value | |
| IsSecure | secure | |
| IsHTTPOnly | httpOnly | |
| ExpireAt | expirationDate | Unix timestamp as float64 |

Non-cookie categories fall back to the standard JSON formatter.

## 4. File Organization

Output follows a **one file per category** convention:

```
results/
├── password.csv
├── cookie.csv
├── history.csv
├── bookmark.csv
├── download.csv
├── creditcard.csv
├── extension.csv
├── localstorage.csv
└── sessionstorage.csv
```

Data from all browser profiles is aggregated into the same file. The `browser` and `profile` columns identify which browser and profile each row came from. Empty categories produce no file.

File permissions are restrictive: directories `0750`, files `0600` (data may contain passwords and cookies).

## 5. Data Flow

```
CLI: hack-browser-data dump -b chrome -c password,cookie -f csv -d results
  → PickBrowsers(name="chrome")       → []Browser
  → parseCategories("password,cookie") → []Category
  → NewWriter("results", "csv")        → *Writer
  → for each browser:
      Extract(categories) → *BrowserData
      Writer.Add(browser, profile, data)
  → Writer.Write()
      → aggregate by category → format rows → write files
  → (optional) CompressDir → results.zip
```

## Related RFCs

| RFC | Topic |
|-----|-------|
| [RFC-001](001-project-architecture.md) | Browser interface and Extract() orchestration |
| [RFC-008](008-file-acquisition-and-platform-quirks.md) | File utilities (CompressDir) |
