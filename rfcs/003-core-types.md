# RFC-003: Core Types

This RFC fixes the shape of the central types; their names and the alternatives behind them are in [RFC-004](004-naming-decisions.md). Go 1.20 — no generics-stdlib helpers, but ordinary type parameters are available and used sparingly where they remove real duplication.

Two rules apply to every type below:

- **Interfaces live with their consumer; constructors return concrete types.** `browser` declares the `Collector` its extractors use, and `app` implements it. `snapshot.Temp` returns a `*snapshot.Dir`, `export.NewWriter` a `*export.Writer`, and `browser.NewProfileSnapshot` a `ProfileSnapshot`, not interfaces.
- **An enum's zero value is its default or "none", never a real non-default choice.** `category.Category`, `browser.Base`, and `app.Outcome` start at 1, so a forgotten field never reads as `Password` or `OutcomeOK`. `export.Format`'s zero is `JSON`, the `-f` default. `masterkey.Version` is a string whose zero is `""`.

## Discovery types — `internal/browser`

```go
// DiscoverOptions is what discovery is handed: everything it would otherwise read from the environment.
type DiscoverOptions struct {
    Home        string // the user's home directory, resolved once by app; tests pass a temp dir
    ProfilePath string // the -p value, already validated; "" when not given (RFC-009)
}

// DirID names one user data dir: Def.ID for the first one of a Def, then "-2", "-3"… appended for further
// ones, in Path order. It is its own type so that it cannot be passed where a Def.ID belongs.
type DirID string

// UserDataDir is one browser data directory found on this machine: Chromium's "User Data",
// Firefox's Profiles directory, Safari's data under ~/Library.
type UserDataDir struct {
    ID       DirID     // "chrome", "arc", "arc-2" — also the snapshot and archive directory name
    Def      Def       // which browser it belongs to (RFC-005)
    Path     string    // absolute path; every Source path resolves under it
    Profiles []Profile
}

// Profile is one profile within a user data dir.
type Profile struct {
    Name string // the output's `profile` value (see below)
    Dir  string // relative to UserDataDir.Path; "" when the user data dir itself is the profile (flat layout)
}
```

`UserDataDir` is Chromium's own term (`--user-data-dir`): the directory holding `Local State` and the profile directories, as opposed to the program installation that holds the executable. HBD uses it for every base's data root — the directory a base's sources resolve under — even where the browser has no such word: Firefox's `Profiles` directory, and Safari's `~/Library` (below). A machine can have several for one browser — Arc's two MSIX packages on Windows each have one — and `restore` works on a copied one that was never installed here, which is why the type is not called `Installation` ([RFC-004](004-naming-decisions.md)).

Two IDs sit side by side and must not be confused: `dir.ID` (`arc-2`) names the directory, `dir.Def.ID` (`arc`) names the browser. The `-b` value, a `keys.json` vault's `browser` key, and the per-fork overrides use `Def.ID`; the snapshot and archive layout use the `DirID`. On a machine with one Arc the two are equal, so a mix-up would pass every test there — the separate type makes it a compile error instead.

`Profile.Name` is frozen by the output contract ([RFC-000](000-goals-and-constraints.md)): for Chromium the directory's base name (`Default`, `Profile 1` — what Chromium's `Profile::GetBaseName()` returns, not the display name `Local State` keeps in `profile.info_cache`); for Firefox the directory name (`97nszz88.default-release`); for Safari, `default` or the named profile's title (deduplicated with a `-N` suffix, the UUID when untitled). It is never a path.

Safari has no single data directory — History lives under `~/Library/Safari`, cookies under `~/Library/Containers/com.apple.Safari/…` or `~/Library/Cookies` — so its user data dir is `~/Library`, the nearest common ancestor.

## Extractor contract — `internal/browser`

```go
// Source is one file or directory an extractor needs.
type Source struct {
    Paths    []string // candidates in priority order, relative to the profile dir — or to the user data dir when Shared
    IsDir    bool     // a directory (LevelDB, Session Storage), copied whole
    Shared   bool     // belongs to the whole user data dir, not one profile: Local State
    Optional bool     // the extraction still runs without it
}

// Extractor declares the sources a category needs and how to parse them.
type Extractor struct {
    Sources []Source
    Extract func(ctx context.Context, ps ProfileSnapshot, out Collector) error
}

// Extractors maps each category a profile supports to its extractor.
type Extractors map[category.Category]Extractor
```

Sources are static data: a base package declares them once per category and never rewrites them per profile. Plan joins a profile-scoped path with the profile's `Dir`, leaves a `Shared` one as it is, and captures the union once per user data dir — so `Local State`, or the Chromium `History` file that feeds both history and downloads, is copied once however many extractions need it.

An extraction is `absent` when a required source has no candidate on disk, or when all its sources are optional and none of the profile's own is there: a shared source such as `Local State` never makes an extraction present by itself. Chromium passwords are two optional sources — `Login Data` (store `local`) and `Login Data For Account` (store `account`) — which reproduces today's output, including a profile that only ever synced passwords, without a special case.

`Extractor` / `Extractors` are **DECIDED**.

### ProfileSnapshot and Collector

```go
// ProfileSnapshot is what an extractor is handed for one extraction: its profile's view of the user data dir's
// snapshot, and the dir's keys.
type ProfileSnapshot struct {
    Profile    Profile // the extraction's profile; profile-scoped sources resolve under its Dir
    snap       snapshot.Snapshot
    masterKeys func(ctx context.Context) masterkey.Set
}

// NewProfileSnapshot is how app hands a profile its share; tests pass a fixture Snapshot and a fake masterKeys.
func NewProfileSnapshot(snap snapshot.Snapshot, p Profile, masterKeys func(context.Context) masterkey.Set) ProfileSnapshot

// Path returns the captured path of src: its first candidate present in the snapshot. A candidate that was
// never captured is skipped; one whose copy failed returns that copy's error.
func (ps ProfileSnapshot) Path(src Source) (string, error)

// MasterKeys returns the user data dir's Chromium keys, one per version. The first call in a dir retrieves them and
// later calls reuse the result; a version that could not be retrieved is absent from the Set.
func (ps ProfileSnapshot) MasterKeys(ctx context.Context) masterkey.Set

// Collector receives one extraction's output. It stamps Browser and Profile on every record, so no extractor can omit
// them.
type Collector interface {
    Add(r record.Record) // one parsed record
    Fail(err error)      // one record-level problem: a secret that could not be recovered, or a row dropped as unparseable
}
```

An extractor reads from its `Source`s and hands its records to the `Collector`. It emits records as it reads them; it does not accumulate slices. A record whose secret can't be recovered is emitted anyway with that field empty — today's output — and reported through `Fail`, which records the problem and lets the extraction continue, like `testing.T.Error`; the extraction ends `partial`. An extractor never logs errors: it returns extraction-level errors and reports record-level ones. Base-specific context an extractor needs (Safari's profile UUID for filtering downloads and the run's login keychain, the Yandex override) is captured when the base package builds the profile's `Extractors`, not passed through the `ProfileSnapshot`.

`MasterKeys` takes a context because its first call may block — on a secret-store lookup, or on macOS on the login keychain's no-echo password prompt — and Ctrl-C cancels the run's context ([RFC-009](009-cli-surface.md)). It returns no error: the dir's call-once in `app` keeps the retrieval error and reports it once, as the user data dir's `error` result ([RFC-001](001-architecture-and-pipeline.md)), and a record whose key is missing is emitted with that field empty and reported through `Fail`. Every `ProfileSnapshot` of one user data dir shares that call-once; `masterKeys` is nil for Firefox and Safari, whose extractors never call it.

There is no separate `Secrets` handle ([RFC-004](004-naming-decisions.md)), and the login password is not on `ProfileSnapshot` at all: it opens one resource, the login keychain, and that resource is what gets handed out (below).

## Pipeline values — `internal/app`

```go
// Extraction is one unit of work: one category of data from one profile of one user data dir. It is pure data —
// the extractor is looked up when it runs — so a Plan prints as it is and compares with reflect.DeepEqual in tests.
type Extraction struct {
    DirID    browser.DirID     // which user data dir
    Profile  string            // Profile.Name; "" in a result about a whole browser or user data dir
    Category category.Category // zero in a result about a whole browser or user data dir
}

// Discovery is what Discover found: the user data dirs, plus an error result for each browser whose data could
// not be read — reported, never mistaken for absence.
type Discovery struct {
    Dirs   []browser.UserDataDir
    Failed []Result
}

type Plan struct {
    Dirs        []browser.UserDataDir // the discovered dirs, in output order
    Files       []snapshot.File       // union of files to capture, deduplicated by (user data dir, path)
    Extractions []Extraction          // what will run, in output order (RFC-008)
    Skipped     []Result              // requested but can't run: unsupported, absent, or Discovery.Failed
}

func Discover(req Request) Discovery
func NewPlan(d Discovery, req Request) Plan
```

`Plan` is a plain value: printable, testable, and all `list` prints. `restore` builds its `Discovery` from the snapshot directory it opens instead of from the machine ([RFC-007](007-snapshot-and-acquisition.md)).

## Outcome & Summary — `internal/app`

```go
type Outcome int

const (
    OutcomeOK Outcome = iota + 1
    OutcomePartial
    OutcomeAbsent
    OutcomeUnsupported
    OutcomeError
)

type Result struct {
    Extraction
    Outcome Outcome
    Count   int   // records emitted
    Failed  int   // Collector.Fail calls
    Err     error // set when Outcome == OutcomeError
}

// Summary is what a run reports: every result, in output order.
type Summary struct {
    Results []Result
}

// Execute runs Capture, Extract, and Export one user data dir at a time (RFC-001). Its error means the run could
// not finish — an output write failure; every other failure is a Result.
func Execute(ctx context.Context, p Plan, req Request) (Summary, error)
```

The outcome follows from the counts: `OutcomeOK` when nothing failed — zero records included, since an empty source is not absent; `OutcomePartial` when records were emitted and some failed; `OutcomeError` when the extractor returned an error, a source failed to copy, or failures were reported with no record emitted. `Summary` starts with `Plan.Skipped`, so the report covers what never ran as well as what did.

The exit status is not derived from `Summary`: it is `0` whenever the run completes and `1` when it cannot start or finish ([RFC-001](001-architecture-and-pipeline.md)). `Summary` feeds the report only.

The stage functions are `Validate`, `Discover`, `NewPlan`, `Execute`, and `Report`; the values they produce are `Request`, `Discovery`, `Plan`, and `Summary`. No function shares a name with a type, which Go would reject.

## Categories — `internal/category`

What kind of browser data something is. Every stage speaks it: the CLI parses `-c` into it, a base package keys its `Extractors` by it, the planner crosses it with profiles, and the exporter names its files after it.

```go
// Category is one kind of browser data. The zero Category means "none".
type Category int

const (
    Password Category = iota + 1
    Cookie
    Bookmark
    History
    Download
    CreditCard
    Extension
    LocalStorage
    SessionStorage
)

// String returns "password", "cookie", "bookmark", "history", "download", "creditcard",
// "extension", "localstorage", "sessionstorage" — also the -c values and the output file names.
func (c Category) String() string

func All() []Category                  // every category, in the order above
func Parse(s string) (Category, error) // the inverse of String, for -c
```

The package is tiny on purpose, like `timeconv`: it exists so that `category.Cookie` and `record.Cookie` can each mean exactly what they say ([RFC-004](004-naming-decisions.md)). There is no `Category.Sensitive()`: with `-c all` as the frozen default, nothing would consume it.

## Record types — `internal/record`

A record is one row of output — one saved login, one cookie, one history entry — and it has the same type whichever browser produced it.

```go
// Login is one saved password: a URL, a user name, and a password together, as Chromium's "Login Data"
// and Firefox's logins.json keep them.
type Login struct {
    Browser   string    `json:"browser" csv:"browser"`
    Profile   string    `json:"profile" csv:"profile"`
    URL       string    `json:"url" csv:"url"`
    Username  string    `json:"username" csv:"username"`
    Password  string    `json:"password" csv:"password"`
    CreatedAt time.Time `json:"created_at" csv:"created_at"`
    Store     Store     `json:"store" csv:"store"`
}

// Store names the Chromium password database a Login came from.
type Store string

const (
    StoreLocal   Store = "local"   // Login Data
    StoreAccount Store = "account" // Login Data For Account
)

// Record is a record of any category: one row of output. It is sealed: only this package's record types implement it.
type Record interface {
    Category() category.Category
    stamp(browser, profile string)
}

// Stamp sets the two source fields every record starts with. The pipeline's Collector calls it on each record it is
// handed; it is exported because the sealing method stamp is not, and an unexported method can only be called from
// its own package.
func Stamp(r Record, browser, profile string)
```

| Category                  | Record type             | Today's type            |
| ------------------------- | ----------------------- | ----------------------- |
| `category.Password`       | `record.Login`          | `types.LoginEntry`      |
| `category.Cookie`         | `record.Cookie`         | `types.CookieEntry`     |
| `category.Bookmark`       | `record.Bookmark`       | `types.BookmarkEntry`   |
| `category.History`        | `record.History`        | `types.HistoryEntry`    |
| `category.Download`       | `record.Download`       | `types.DownloadEntry`   |
| `category.CreditCard`     | `record.CreditCard`     | `types.CreditCardEntry` |
| `category.Extension`      | `record.Extension`      | `types.ExtensionEntry`  |
| `category.LocalStorage`   | `record.LocalStorage`   | `types.StorageEntry`    |
| `category.SessionStorage` | `record.SessionStorage` | `types.StorageEntry`    |

Every record type starts with two flat source fields, `Browser string` (the display name) and `Profile string` (`Profile.Name`), followed by today's fields with today's names, tags, and order. `LocalStorage` and `SessionStorage` declare the same fields separately, so each is its own type with its own category. Records are used by pointer — `out.Add(&record.Cookie{…})` — because `stamp` sets the fields in place. There is no `Origin`/`Owner` umbrella type ([RFC-004](004-naming-decisions.md)).

Only the type names change. Field names are wire names — each field's snake_case form, enforced by lint — and because the output is frozen, so are they ([RFC-008](008-output-and-export.md)). `Store` becomes a named string type with today's two values, which changes nothing on the wire.

## Master key — `internal/masterkey`

```go
// Version is a Chromium ciphertext's version prefix.
type Version string

const (
    V10 Version = "v10"
    V11 Version = "v11"
    V20 Version = "v20"
)

// Set holds one Chromium user data dir's key per version. Its JSON form is today's keys.json
// "keys" object: {"v10": …, "v11": …, "v20": …}.
type Set map[Version][]byte

// Retriever obtains one version's key from one OS secret store. Implementations are
// build-tagged and return ErrNotApplicable when they don't apply to this user data dir.
type Retriever interface {
    Version() Version
    Retrieve(ctx context.Context, loc Locator) ([]byte, error)
}

// RetrieveAll runs this OS's retrievers in order. Per version the first key wins and later retrievers for that
// version are not tried; a version's error is reported only when all its retrievers fail. Versions are
// independent, so a v20 failure never discards a v10 key.
func RetrieveAll(ctx context.Context, loc Locator) (Set, error)
```

`Set` / `RetrieveAll` are **DECIDED** ([RFC-004](004-naming-decisions.md)). Because `Version` is a string type, `encoding/json` writes a `Set` with today's `"v10"` keys, sorted, and no marshaling code. Key scopes, retriever order, and laziness are in [RFC-006](006-key-retrieval-and-crypto.md).

## Login keychain — `internal/keychain`

```go
// PasswordFunc yields the macOS login password. app builds it from --keychain-pw, --keychain-pw-stdin, or the prompt
// (RFC-009); outside this package it is the only place the password exists.
type PasswordFunc func(ctx context.Context) (string, error)

// Keychain is the user's login keychain. It opens on first use, at most once per run.
type Keychain struct{ /* unexported */ }

// ErrLocked reports that no usable password was supplied, so the keychain yields metadata only.
var ErrLocked = errors.New("login keychain is locked")

func New(path string, password PasswordFunc) *Keychain // no I/O: building one never prompts

func (k *Keychain) GenericPassword(ctx context.Context, account string) ([]byte, error)               // ErrLocked when locked
func (k *Keychain) InternetPasswords(ctx context.Context) ([]keychainbreaker.InternetPassword, error) // secrets empty when locked
func (k *Keychain) Err() error                                                                        // why it is locked; nil if open or never used
```

The login keychain belongs to the user, not to a profile or a user data dir, so one handle serves the whole run. On macOS `app` builds it and hands it to its two consumers: the Chromium `v10` retriever, through `masterkey.Locator`, and the Safari password extractor, which `safari.Extractors` captures for the default profile. Each consumer declares the one method it reads (`masterkey.LoginKeychain`, `safari.LoginKeychain`), so its tests need no real keychain. Opening the keychain is the password check: a missing or wrong password leaves it locked — metadata only, as today — and `Err` gives the reason, which the reporter prints once. The package builds only on macOS ([RFC-002](002-package-layout.md)); elsewhere, and in `restore`, there is no handle. Retrieval order and laziness are in [RFC-006](006-key-retrieval-and-crypto.md).

## Snapshot — `internal/snapshot`

```go
// File is one file or directory a plan wants captured.
type File struct {
    DirID string // the user data dir's ID ("chrome", "arc-2"); also the snapshot it lands in
    From  string // absolute path of the user data dir it is copied from
    Rel   string // slash-separated path relative to From, and to the snapshot
    IsDir bool
}

// Dir is a snapshot directory: one Snapshot per user data dir.
type Dir struct{ /* unexported */ }

func Temp() (*Dir, error)            // a new temporary snapshot directory, mode 0700
func Open(path string) (*Dir, error) // an existing one, used in place (restore --data-dir)
func Unzip(src string) (*Dir, error) // restore transport into a temporary snapshot directory; rejects entries that escape it

func (d *Dir) Capture(ctx context.Context, files []File) error // copies what it can; per-file failures surface through Snapshot.Path
func (d *Dir) Snapshot(dirID string) Snapshot
func (d *Dir) Zip(dst string) error                            // archive: temp file beside dst, renamed into place on success
func (d *Dir) Close() error                                    // removes a temporary snapshot directory; leaves an opened one in place

// Snapshot is one user data dir's snapshot: the read-only view of the files captured from it.
type Snapshot struct{ /* unexported */ }

// Path returns rel's absolute path inside the snapshot: fs.ErrNotExist when the file was never captured or had
// vanished before Capture reached it, or the copy's error when capturing it failed.
func (s Snapshot) Path(rel string) (string, error)
```

`Capture` returns an error only when it cannot go on — a cancelled context, an unwritable snapshot directory. A single file's failure is kept and handed back by `Snapshot.Path`, so it ends that file's extractions as `error` without stopping the rest. `snapshot` cannot import `browser` (`browser.ProfileSnapshot` holds a `snapshot.Snapshot`), so a `File` names its user data dir with a plain string, `DirID`. `Snapshot` hands out real paths because `modernc.org/sqlite` and `goleveldb` open files by path, and SQLite needs its `-wal` file beside the database. The snapshot is therefore always a directory; zip is only a transport format ([RFC-007](007-snapshot-and-acquisition.md)).

## Export — `internal/export`

```go
// Format is the -f value. Its zero value is JSON, the default.
type Format int

const (
    JSON Format = iota
    CSV
    CookieEditor
)

func ParseFormat(s string) (Format, error) // "json", "csv", "cookie-editor"

// Writer appends each extraction's records to today's per-category files, in plan order.
type Writer struct{ /* unexported */ }

func NewWriter(dir string, f Format, zip bool) (*Writer, error)
func (w *Writer) Write(c category.Category, records []record.Record) error // one extraction's records; the writer sorts them (RFC-008)
func (w *Writer) Close() error                                             // finishes every file; with --zip, packs them
```
