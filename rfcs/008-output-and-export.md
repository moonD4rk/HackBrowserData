# RFC-008: Output & Export

Everything a user can see in the output — file names, formats, field names, stamp values — is frozen by the compatibility contract ([RFC-000](000-goals-and-constraints.md)). This RFC describes how the rewrite reproduces it byte for byte, and the few places where today's output is wrong and gets fixed.

## Contract

```go
type Format int // the -f value; the zero value is JSON, the default

const (
    JSON Format = iota
    CSV
    CookieEditor
)

func ParseFormat(s string) (Format, error) // "json", "csv", "cookie-editor"

type Writer struct{ /* unexported */ }

func NewWriter(dir string, f Format, zip bool) (*Writer, error)
func (w *Writer) Write(c category.Category, records []record.Record) error // one extraction's records, appended in plan order
func (w *Writer) Close() error                                             // finishes every file; with --zip, packs them
```

Export writes one file per non-empty category: `password.csv`, `cookie.json`, and so on — `<category>.<ext>`, where the extension is `csv` or `json` (`cookie-editor` writes `.json`). A category with no records produces no file. The output directory is created with mode `0750` and each file with `0600`, as today. One `-f` value per run, as today.

## Formats

- **`json`** (default) — one array of objects per category, two-space indent, UTF-8 without a BOM. Strings keep today's HTML escaping: `&`, `<`, and `>` are written as `\u0026`, `\u003c`, and `\u003e`. Today's per-row `MarshalJSON` calls `json.Marshal`, which escapes them even though the outer encoder turns escaping off; the new records are plain structs, so the encoder leaves escaping on to produce the same bytes. Any URL with a query string shows the difference, so the parity suite catches a slip at once.
- **`csv`** — a header row, then one row per record; UTF-8 with a 3-byte BOM so Excel reads non-ASCII correctly; times in RFC 3339 (empty when zero), booleans as `true` / `false`.
- **`cookie-editor`** — cookies as the Cookie-Editor extension's import array (`expirationDate`, `httpOnly`, `hostOnly`, `sameSite`, …), written without HTML escaping, as today; every other category falls back to the `json` encoder above, which is what today's code does.

Adding a format later is a new `Format` value and a new encoder; nothing else in the pipeline changes.

## Wire names follow Go names (DECIDED)

JSON keys and CSV headers are the snake_case form of the Go field names: `Cookie.IsHTTPOnly` is `is_http_only`, `Extension.HomepageURL` is `homepage_url`. `encoding/json` does not derive names on its own, so every field keeps an explicit `json` and `csv` tag, and the rule "tag = snake_case(field name)" is enforced by `tagliatelle` (`use-field-name: true`, `json: snake`, `csv: snake`). Today's record and `keys.json` types already pass that check unchanged; the Cookie-Editor encoder is exempt, because its keys are the extension's import schema, not ours.

The consequence is stated plainly: since the output is frozen, **the record field names are frozen too** — a rename would change a column. A field can only be renamed together with a deliberate, announced output change, and the lint makes such a change impossible to miss in review.

### Source stamp

Every record starts with two flat source fields — `Browser` and `Profile` ([RFC-003](003-core-types.md)) — serialized as the leading `browser` and `profile` keys in JSON and the leading columns in CSV, exactly as today. `browser` is the display name (`Chrome`, `Microsoft Edge`); `profile` is the profile name (`Default`, `Profile 1`, a Safari profile's title), never a path. The pipeline's `Collector` sets both through `record.Stamp`, so no extractor can leave them out.

## Deterministic ordering

Output order is defined once per category, in the exporter, for every browser. Files list extractions in plan order — the browser table's order (one merged list whose per-OS projections equal today's `platformBrowsers()` orders), then user data dir ID, then profiles in discovery order (directory order for Chromium and Firefox, the default profile first for Safari) — which matches today's order. Within an extraction, records are sorted by today's key, with tie-breakers added and a stable sort (`sort.SliceStable`; Go 1.20 has no `slices`), so two runs over the same snapshot produce byte-identical files:

| Category                                            | Order                                                                                  |
| --------------------------------------------------- | -------------------------------------------------------------------------------------- |
| password                                            | `created_at` descending, then `store` (local before account), `url`, `username`        |
| cookie                                              | `created_at` descending, then `host`, `name`, `path`                                   |
| history                                             | `visit_count` descending, then `last_visit` descending, `url`                          |
| download                                            | `start_time` descending, then `url`                                                    |
| bookmark                                            | `created_at` descending, then `id` ascending                                           |
| creditcard, extension, localstorage, sessionstorage | source order, made explicit (`ORDER BY rowid`, LevelDB key order, JSON document order) |

Today the order is decided in each extractor with an unstable `sort.Slice`, so ties come out in arbitrary order — and Firefox history sorts _ascending_ by visit count (`browser/firefox/extract_history.go`) while Chromium and Safari sort descending. The table above keeps today's keys and fixes Firefox to match the others.

## Streaming vs. buffering

Extractors emit records one at a time ([RFC-003](003-core-types.md)), but sorting needs an extraction's full set. The writer buffers one extraction's records, sorts them, appends them to that category's file, and releases them — memory is bounded by the largest single category of a single profile, where today's writer holds the whole run. The standard encoder cannot append to an open array, so the writer frames it itself: `[` before the first element, each element encoded with a two-space prefix and indent, `,` between elements, and `]` plus a newline on `Close`. Those are the bytes a single `Encode` of the whole slice produces, and the parity suite proves it ([RFC-011](011-testing-and-quality.md)).

## `--zip`

`--zip` keeps today's result — `<dir>/<basename(dir)>.zip` with the files at its top level, and the loose files removed — but fixes the order. Today `fileutil.CompressDir` deletes each file as soon as it is added to an in-memory zip and writes the zip to disk last, so any failure along the way loses the output. The rewrite writes the zip to a temp file, closes and renames it into place, and only then deletes the originals; a failure leaves every output file where it was.

## Output fixes (listed exceptions to "unchanged")

- **Firefox history order** — `visit_count` descending, like Chromium and Safari.
- **Firefox bookmarks** — `folder` holds the real folder name instead of `url`, and `type` and `id` are filled; today they are always `url`, empty, and `0`.
- **Safari bookmark IDs** — unique across the whole bookmark tree; today each ID is the index among its siblings, so IDs repeat across folders.
- **`browser` in `restore` output** — the display name (`Chrome`), as `dump` writes it; today `restore` writes the vault key (`chrome`).

## Secrets in output

`-c all` is the default, as today, so a bare run exports every category, passwords and cookies included. The exporter does not redact: what reaches Export was asked for.

## DEFERRED

- **JSON without HTML escaping.** Every JSON parser reads `\u0026` and `&` as the same string, but dropping the escapes would change bytes in nearly every file, so it waits for an announced output change.
