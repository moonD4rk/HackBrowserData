# RFC-004: Naming Decisions

This is the one place the naming discussion lives. Each item keeps its rationale and the alternatives rejected, so the decisions are auditable rather than bare assertions.

Go naming principles applied throughout:

- **The package name is part of the identifier.** `category.Cookie` already says "the cookie category," so a `Category` prefix or a `record.Record`-style stutter is redundant. Names are judged at the call site, not in isolation.
- **A qualified name reads as what it is.** `category.Cookie` is a category and `record.Cookie` is a cookie record; neither needs a suffix to tell it from the other, because each lives in the package that means it.
- **A name says what a thing is, not how it is built or used, or what it resembles.** `ProfileSnapshot`, not `Input`; `Snapshot`, not `Tree`.
- **One concept, one name.** A user data dir's location is a `Path`, its identity a `DirID`, a ciphertext's prefix a `Version` — in code and in prose alike.
- **Values are nouns, stages are verbs.** A Go package cannot hold a type and a function of the same name, so the pipeline's values (`Plan`, `Summary`) and the stages that produce them (`NewPlan`, `Execute`) never share a word.

Internal Go names are free to change; anything the user can see — commands, flags, file names, field names, `keys.json` keys — is frozen by the compatibility contract ([RFC-000](000-goals-and-constraints.md)), whatever the Go name behind it.

## Settled (DECIDED)

| Name                                                                                                     | Decision                                                                                                                                                        |
| -------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `internal/` for all non-`cmd` code                                                                       | [RFC-002](002-package-layout.md).                                                                                                                               |
| `browser.Extractor` / `browser.Extractors`                                                               | The thing that declares sources and parses them.                                                                                                                |
| `masterkey.Set` / `masterkey.RetrieveAll`                                                                | The key collection and the function that fills it.                                                                                                              |
| Wire names = snake_case of Go field names                                                                | No second vocabulary; lint-enforced; frozen with the output. [RFC-008](008-output-and-export.md).                                                               |
| `browser` stays `browser` (not `engine`)                                                                 | "Engine" means a web rendering engine (Blink/Gecko/WebKit) and misleads.                                                                                        |
| `browser.Supported()` / `browser.Lookup`                                                                 | This OS's browsers / any row of the whole table. No `catalog` package.                                                                                          |
| `category.Category` with bare values; records `record.Login`, `record.Cookie`…; sum type `record.Record` | Categories and records in separate packages, so neither needs a suffix; no `model` package; the sum type takes its package's name, as `category.Category` does. |
| Per-record source: flat `Browser` + `Profile`                                                            | No `Origin`/`Owner` type; the fields hold v1.1.0's output values.                                                                                               |
| `browser.ProfileSnapshot`                                                                                | What an extractor is handed: its profile's view of the snapshot and the user data dir's keys (`Path`, `MasterKeys`); no `Secrets` handle, not `Input`.          |
| `keychain.Keychain`                                                                                      | The run's macOS login keychain: one handle, opened on first use; consumers get the handle, never the password.                                                  |
| `browser.Collector` (`Add`, `Fail`)                                                                      | What an extractor hands its records and record-level problems to; not `Sink`, not `Emit`.                                                                       |
| `masterkey.Locator` / `chromium.KeyLocator`                                                              | Where one user data dir's keys are found, and the function that builds it; not `RetrieveInput`.                                                                 |
| `Base` enum; product identity in `Def.ID`                                                                | No `Variant` enum; channel is folded into rows, not modelled.                                                                                                   |
| `browser.UserDataDir` with a `DirID`                                                                     | Not `Installation`; the ID has its own type.                                                                                                                    |
| `app` stages and values                                                                                  | `Validate`, `Discover`, `NewPlan`, `Execute`, `Report`; `Request`, `Discovery`, `Plan`, `Extraction`, `Summary`.                                                |
| `snapshot.Dir` / `snapshot.Snapshot`                                                                     | A snapshot directory and one user data dir's snapshot inside it, a snapshot being one user data dir's copy as in Chromium; not `FS`, `Tree`, or `Copy`.         |
| `winapi`, `abe`; `log` and `crypto` kept                                                                 | Windows-only packages; not `platform/windows`.                                                                                                                  |
| `timeconv` and `sqlitex` functions                                                                       | `timeconv.Chromium`, `UnixMicro`, `UnixMilli`, `Unix`, `CoreData`, each named for the encoding it reads; `sqlitex` exports `CountRows` and `QueryRows` only.    |
| CLI names unchanged                                                                                      | `-c/--category` selects `category.Category` values; `-p/--profile-path` and `dumpkeys` stay. [RFC-009](009-cli-surface.md).                                     |

## Item-by-item detail

### 1. The supported-browser table — `browser.Supported()` and `browser.Lookup`

**Decision.** The table lives in `browser`, where it belongs; there is no `catalog` package.

```go
browser.Supported() []browser.Def      // the browsers HBD supports on the running OS, in table order
browser.Lookup(id string) (Def, bool)  // any row of the whole table, whatever the OS
```

`Supported()` is exactly what its callers need: Discover walks it, `-b` is validated against it, and the help text lists it. It reads as "the browsers that are supported," and on macOS that list must not contain `qq` or `sogou`, which have no macOS row. `restore` needs the other view — a vault taken on Windows can name a Windows-only fork — so it resolves IDs with `Lookup`, which searches the whole table. No caller needs the whole table as a list; tests that do read the package's unexported table.

**Alternatives rejected.**

| Option                                  | Why not                                                                                     |
| --------------------------------------- | ------------------------------------------------------------------------------------------- |
| `catalog.All()`                         | "all of the catalog" says nothing about _what_; the `catalog` noun doesn't pull its weight. |
| `registry.All()`                        | "registry" is implementation jargon; same empty `All()`.                                    |
| `catalog.Browsers()`                    | keeps the `catalog` noun.                                                                   |
| `browser.Known()`                       | an honest name for the whole table, but no caller wants the whole table as a list.          |
| `Supported()` returning every OS's rows | the name would promise "supported here" and deliver "known anywhere."                       |

### 2. Per-record source — flat `Browser` + `Profile`, no umbrella type

**Decision.** Each record carries two flat fields — `Browser string` and `Profile string` — not a wrapper type. `Browser` holds `Def.Name`, the display name (`Chrome`, `Microsoft Edge`), and `Profile` holds `Profile.Name` (`Default`, or a Safari profile's title) — exactly the values v1.1.0's `browser` and `profile` columns carry. Putting `Def.ID` (`chrome`) in `Browser` would change every row, which the compatibility contract rules out.

**Why no umbrella type.** `Owner` is wrong (it implies possession/ACLs). `Origin` is worse in this domain: "origin" is already the Web-platform term (`scheme://host:port`), so `cookie.Origin` reads as "the site this cookie belongs to," not "where I pulled it from" — the same collision that got `engine` rejected. `Provenance` is the DFIR term but reads heavy. The browser domain has no clean single noun for "(browser, profile) a datum came from", so the record stamps a `browser` and a `profile` column, as v1.1.0's output already did.

**`Profile` holds the name, never a path.** This keeps local absolute paths out of exported data and keeps archives portable across hosts. It does not collide with the `browser.Profile` type or with `Profile.Dir` (the path) — a field named `Profile` of type `string` lives in a different namespace from the type.

**Stamping.** The fields are exported so `encoding/json` serializes them, but extractors don't set them: the pipeline's `Collector` calls `record.Stamp` on every record it is handed. `Stamp` is exported because `Record`'s sealing method `stamp` is not, and Go only lets the declaring package call an unexported method.

On the wire: `browser` and `profile` keys in JSON; leading `browser` / `profile` columns in CSV.

**Alternatives rejected.**

| Option            | Why not                                                          |
| ----------------- | ---------------------------------------------------------------- |
| `Origin` type     | collides with Web-platform "origin" in a browser-data tool.      |
| `Provenance` type | accurate (DFIR) but heavy, and still an invented umbrella.       |
| `Source` type     | `browser.Source` is the extractor's file set; two "Source"s mix. |

### 3. The extractor's input — `browser.ProfileSnapshot`

**Decision.** An extractor is handed a `browser.ProfileSnapshot`: its profile's view of the user data dir's snapshot, plus the dir's Chromium keys. It is a concrete type with two methods, built by `app` through `NewProfileSnapshot`:

```go
func (ps ProfileSnapshot) Path(src Source) (string, error)              // where src was captured in the snapshot
func (ps ProfileSnapshot) MasterKeys(ctx context.Context) masterkey.Set // this user data dir's Chromium keys; the first call retrieves
```

**Why `ProfileSnapshot`.** It names what the extractor holds — one profile's share of the snapshot — where `Input` named only a role: every function has inputs. `ps.Path(loginData)` and `ps.MasterKeys(ctx)` read as what they return.

**Why no `Secrets` handle.** A `Secrets` interface wrapping the keys and the macOS login password was considered. Master keys and the login password have nothing in common but being secret; the domain has no clean noun for both, and inventing one is the fault item 2 rejects for `Origin` and `Owner`. Without the password (item 8), `Secrets` would be a one-method interface that `app` implements and every extractor reaches through; the method sits better on the value the extractor already holds. The laziness stays where it was — a per-user-data-dir call-once in `app`, passed in as a function — so extractors depend on having keys, not on how they are fetched.

**Why no error.** `MasterKeys` returns only the `Set`. A retrieval failure concerns the whole user data dir, so `app`'s call-once reports it once, as that dir's `error` result; the extractor finds the version's key missing and reports the affected records through `Collector.Fail`, as it would any secret it cannot recover.

**Alternatives rejected.**

| Option                                          | Why not                                                                                                                                 |
| ----------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------- |
| `browser.Input`                                 | names a role, not a thing.                                                                                                              |
| `browser.Secrets{MasterKeys, KeychainPassword}` | an invented umbrella over two unrelated secrets (item 2's rule); the password does not belong on the extractor's input at all (item 8). |
| `masterkey.Lazy`                                | names a strategy, not what the extractor holds.                                                                                         |
| `masterkey.Keys{Set()}`                         | `in.Keys.Set()` reads as a setter in Go (`flag.Value.Set`, `http.Header.Set`), and `masterkey.Keys` stutters.                           |

### 4. Browser classification — `Base`, with product identity in `ID`

Anchored in Chromium's own vocabulary (`install_modes.h` calls the product axis _brand_; the older `BrowserDistribution` called it _distribution_).

**Decision.**

- **`Base`** is the single classification enum: `Chromium`, `Firefox`, `Safari`. It is the "what is this built on" axis — "Chromium-based" — and it decides the storage/decryption model and which base package handles it. What these browsers share is the **Chromium platform/codebase**, not the rendering engine (Blink), which is exactly why `engine` would be both a collision and imprecise.
- **Product/brand identity lives in `Def.ID`** (a string, e.g. `"chrome"`, `"edge"`). There is **no `Variant` enum.** Modelling the product axis as an enum would force a code change for every new fork, which fights the "add a fork = add a data row" goal ([RFC-005](005-browser-definitions.md)); a string `ID` keeps forks as pure data. The few forks with their own on-disk format (Yandex, the Opera family) get an override inside the Chromium package keyed by `Def.ID` — still no enum, and a plain fork stays a pure row.
- **Channel** (stable/beta/dev/canary) is a separate axis in Chromium (install modes), but HBD gives it no field, because nothing dispatches on it: a channel that keeps its own user data dir is simply its own row, whose `ID` names product and channel together (`chrome-beta`).

**Alternatives rejected.**

| Axis    | Option                   | Why not                                                                           |
| ------- | ------------------------ | --------------------------------------------------------------------------------- |
| base    | `Family`                 | common informal word, but not domain-native; `Base` reads "Chromium-based".       |
| base    | `Engine`                 | rejected project-wide; also imprecise (renderer ≠ storage model).                 |
| base    | `Platform`               | means OS, not codebase — it is the per-OS half of a `Def` (item 6), not the base. |
| product | `Variant` / `Brand` enum | a per-fork enum fights "fork = data row"; identity belongs in `ID`.               |
| channel | a `Channel` field        | nothing reads it; the row and its `ID` already say it.                            |

### 5. Data categories and records — `category` and `record`

**Decision.** Two small packages, each with bare names:

```go
category.Password, category.Cookie, category.Bookmark, category.History,
category.Download, category.CreditCard, category.Extension,
category.LocalStorage, category.SessionStorage

record.Login, record.Cookie, record.Bookmark, record.History,
record.Download, record.CreditCard, record.Extension,
record.LocalStorage, record.SessionStorage
```

A category says what kind of data is meant; a record is one row of that data. The two share their nouns — a cookie category, a cookie record — so in one package one of them would need a suffix, and a reader of `record.Cookie` would take a category for a record. In two packages each qualified name reads as what it is: `extractors[category.Cookie]`, `out.Add(&record.Cookie{…})`, and `-c cookie` → `category.Parse` → `category.Cookie` → `cookie.json`. The `Category.String()` values (`password`, `creditcard`, `localstorage`, …) are the `-c` values and the output file names, so they are frozen.

**Why `Category`, not `Kind`.** The CLI already calls it a category — `-c/--category`, "data categories" in the help text — and so did v1.1.0's code (`types.Category`), so one concept keeps one name from the flag to the type. "Kind" already means other things here: the frozen `kind` field of a `keys.json` vault (`chromium-yandex`), v1.1.0's `types.BrowserKind`, and `masterkey.ABEKind`. The constants stay bare: the `CategoryCookie` prefix is what fought the package name, not the type's name. The type reads `category.Category`, the accepted Go form for a package's main type (`time.Time`); a variable holding one is `c`, since `category` would shadow the package. Numbering starts at 1, so the zero `Category` means "none": a result about a whole user data dir has no category, and an unset field never reads as `Password`.

**Why `Login` for the password record.** The category is `password`, a frozen `-c` value, but the record holds a URL, a user name, and a password together — Chromium's "Login Data", Firefox's `logins.json`, v1.1.0's `LoginEntry` — and a `record.Password` with a `Password` field would read `p.Password`. Every other record shares its category's name.

**Why `Record` for the sum type.** Every record type implements `record.Record`, so `record.Cookie` reads as a kind of `record.Record`. The type takes its package's name for the reason `category.Category` does — the accepted Go form for a package's main type (`time.Time`); `Item` said nothing about what it holds.

**Alternatives rejected.**

| Option                                                   | Why not                                                                                                      |
| -------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------ |
| `record.Kind` beside `record.CookieItem`… in one package | `record.Cookie` would be a category, not a record, and every record would carry an `Item` suffix.            |
| `Category` + `CategoryCookie`…                           | the prefix repeats the package's meaning at every call site.                                                 |
| `Kind`                                                   | a Go-insider word, and "kind" already names a vault's browser variant and the ABE dispatch.                  |
| `record.Password`                                        | `p.Password` stutters; the row is a login.                                                                   |
| package `model`                                          | says nothing about what it models.                                                                           |
| sum type `record.Item`                                   | says nothing about what it holds; the `time.Time` form that gives `category.Category` gives `record.Record`. |

### 6. The data-directory type — `browser.UserDataDir`, not `Installation`

**Decision.** The type for one browser data directory found on a machine is `browser.UserDataDir`, identified by a `browser.DirID`.

**Why not `Installation`.** In both Chromium and Firefox, an "installation" is the _program_ — the executable and its files. The thing this type holds is the _data_ directory: Chromium's `User Data` (the `--user-data-dir` the browser documents), Firefox's `Profiles` directory, Safari's data under `~/Library`. Three reasons `Installation` is wrong for it:

1. It collides with the program sense already in the table: `Platform.Exe` and `ExeFallbacks` point at the _installation_. One document should not use "install" for two different things.
2. A machine can have several data directories for one installed program (Arc's two MSIX packages), so "one installation" misdescribes the count.
3. In `restore` the directory was copied from another host and was never installed here at all.

`UserDataDir` is Chromium's own term, and it already appeared in v1.1.0's code (`BrowserConfig.UserDataDir`) and in the frozen `keys.json` field `user_data_dir` — so the type name aligns with a word the codebase and the output already use. Firefox and Safari have no such word; for them it means the directory their sources resolve under ([RFC-003](003-core-types.md)).

**Why `DirID` is its own type.** A user data dir's ID (`arc-2`) and its browser's `Def.ID` (`arc`) are equal for the first directory of every browser, so code that passes one where the other belongs passes every test on a machine with one Arc. As distinct types, the mix-up does not compile.

**Supporting names.**

| Name                                   | What it names                                                                                                                                    | Why this name                                                                                                                                                                                                          |
| -------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------ | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `UserDataDir.ID` (`DirID`)             | `chrome`, `arc`, `arc-2` — one per data directory found                                                                                          | Distinguishes two user data dirs of one `Def`; also the snapshot and archive directory name.                                                                                                                           |
| `UserDataDir.Def`                      | the directory's `Def`                                                                                                                            | A field named for its type, the Go norm; `Browser` would clash with the record's `Browser string`.                                                                                                                     |
| `UserDataDir.Path`                     | its absolute path                                                                                                                                | The one word for a user data dir's location; `Root` was rejected as a second word for it.                                                                                                                              |
| `Platform.UserDataDirs`                | where one browser's user data dirs live on one OS                                                                                                | Each match becomes a `UserDataDir`; `Roots` would be another word for the same thing.                                                                                                                                  |
| `Platform` fields                      | `UserDataDirs`, `KeychainLabel`, `Exe`, `ExeFallbacks`, `ABE`                                                                                    | The per-OS half of a `Def`; absorbs `winutil.Entry`. `Platform` means OS here (where `Base` means codebase), so the two never collide.                                                                                 |
| `browser.Source` / `browser.Collector` | the files an extractor reads / what it hands its records to                                                                                      | Says what it does with records, where `Sink` is dataflow jargon. `Add` is Go's verb for putting something into a container (`Emit` is event-framework vocabulary); `Collector.Fail` is what makes `partial` countable. |
| `Source.Shared`                        | a source of the whole user data dir (`Local State`)                                                                                              | Explicit data; the alternative made correctness depend on the order of builder calls ([RFC-005](005-browser-definitions.md)).                                                                                          |
| `browser.DiscoverOptions`              | the home dir and `-p` handed to Discover                                                                                                         | v1.1.0's name; kept.                                                                                                                                                                                                   |
| `masterkey.Version`                    | `v10`, `v11`, `v20`                                                                                                                              | Chromium's own word (`kEncryptionVersionPrefix`); a string type, so a `Set` marshals to the same `"v10"` keys as v1.1.0 with no extra code. "Tier" is not used.                                                        |
| `masterkey.ABEKind`                    | `ABENone`, `ABEChromeBase`, `ABEEdge`, `ABEAvast`                                                                                                | v1.1.0's `winutil.ABEKind`, moved to the package that consumes it.                                                                                                                                                     |
| `masterkey.Locator`                    | where one user data dir's keys are found: the `Local State` copy, the keychain label, the Windows `Exe` and `ABE` kind, the run's login keychain | Names what it holds; `RetrieveInput` named only a role, the fault item 3 rejects in `Input`, and `Hints` sounds optional. `chromium.KeyLocator` builds one.                                                            |

### 7. Pipeline and package names — `app`, `snapshot`, `winapi`

**`app`: nouns for values, verbs for stages.** `Validate` produces a `Request`, `Discover` a `Discovery`, `NewPlan` a `Plan`, `Execute` a `Summary`, and `Report` writes the summary out. `Run` is not used as a type name: in Go it reads as a verb (`cmd.Run`, `t.Run`).

**`Extraction`, not `Job`.** A plan's unit of work is one extraction: one category of data from one profile of one user data dir. `Job` beside `Result` is the vocabulary of Go's worker-pool idiom — `jobs` and `results` channels — so a reader looks for goroutines and a queue that do not exist. `Extraction` names what HBD does, extract browser data, and pairs with `Extract` and `Extractor` without suggesting concurrency. It sits one package away from `browser.Extractors`, a near-homonym, which the package names keep apart.

**`snapshot.Dir` and `snapshot.Snapshot`, not `snapshot.FS`, `snapshot.Tree`, or `snapshot.Copy`.** A Go reader takes `FS` for `io/fs.FS`; this type is not one, and it returns operating-system paths because SQLite and LevelDB open files by path. A `Snapshot` is one user data dir's snapshot, in Chromium's own sense of the word: its `downgrade::SnapshotManager` takes a snapshot by copying selected files of one user data dir. A `Dir` is the snapshot directory that holds one `Snapshot` per user data dir, and `Dir.Snapshot(dirID)` returns one — a method named for what it returns, as `http.Request.Cookie(name)` is. `ProfileSnapshot` is then one profile's share of a `Snapshot`, so the vocabulary closes. Both are concrete types, and the snapshot directory's lifecycle — created with mode `0700`, removed by `Close` — lives in `Dir`'s methods rather than in free functions over a named string. `Zip` and `Unzip` are the transport pair.

**`winapi` and `abe`, not `platform/windows`.** A package named `windows` would collide with `golang.org/x/sys/windows`, which it imports, so every file would need an alias. `winapi` is v1.1.0's name; the ABE bridge is its own unit, `abe`.

**`log` and `crypto` keep their names.** Both share a name with a standard-library package, but both are internal and no file needs both, and both are v1.1.0's names.

**Alternatives rejected.**

| Option                                           | Why not                                                                                                                                                                           |
| ------------------------------------------------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `type Plan` + `func Plan`, `type Run`            | a type and a function can't share a name; `Run` reads as a verb.                                                                                                                  |
| `Job`, `Task`                                    | Go's worker-pool vocabulary (`jobs` / `results` channels); readers look for a queue.                                                                                              |
| `Target`                                         | reads as an attack target in a security tool.                                                                                                                                     |
| `Unit`, `Item`, `Entry`                          | `Unit` and `Item` say nothing; `…Entry` named v1.1.0's record types.                                                                                                              |
| `snapshot.FS`                                    | reads as `io/fs.FS`.                                                                                                                                                              |
| `snapshot.Tree`                                  | a metaphor for its shape, not a name for what it holds.                                                                                                                           |
| `snapshot.Files`                                 | collides with `snapshot.File`, a capture request, and `Plan.Files`.                                                                                                               |
| `snapshot.Copy`                                  | reads as a verb in Go (`io.Copy`, the `copy` builtin), next to `Capture`, the method that does copy.                                                                              |
| `Dir.CopyOf`, `Dir.Get`, `Dir.Lookup`, `Dir.Sub` | `CopyOf` reads as making a copy (Java's `Arrays.copyOf`); `Get` and `Lookup` say less than a method named for what it returns; `Sub` brings back the subtree metaphor (`fs.Sub`). |
| `type Dir string` with free functions            | the snapshot directory's lifecycle belongs on the type.                                                                                                                           |
| `platform/windows`                               | collides with `golang.org/x/sys/windows`.                                                                                                                                         |
| `logx`                                           | renames v1.1.0's package for no reader's benefit, and `crypto` would then need the same treatment.                                                                                |

### 8. The macOS login keychain — `keychain.Keychain`

**Decision.** The login keychain is a resource with one owner, the macOS-only package `internal/keychain`. `keychain.New(path, password)` returns a `*keychain.Keychain` that opens on first use, at most once per run. The macOS Chromium `v10` retriever and the Safari password extractor receive the handle; the password itself goes only from `keychain.PasswordFunc` into the keychain. The type reads `keychain.Keychain`, the accepted Go form for a package's main type (`time.Time`), as `category.Category` does.

**Why a handle, not a password.** In v1.1.0 the password traveled as a string — through `DiscoverOptions`, a setter interface, the retriever constructor, and four Safari function signatures — and every consumer reopened the keychain to use it. A password that was checked and then passed on as a string has to be trusted downstream; an opened keychain carries its own answer: open, or locked with a reason. The keychain belongs to the user, so its scope is the run; as a method on the per-profile extractor input it would put a run-scoped, macOS-only concept on a cross-platform, per-profile type. Each consumer declares the one method it uses — `masterkey.LoginKeychain`, `safari.LoginKeychain` — under the same name, since both name the same thing.

**Alternatives rejected.**

| Option                                         | Why not                                                                                                                                |
| ---------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------- |
| `ProfileSnapshot.KeychainPassword(ctx)`        | hands out the credential instead of the resource; every consumer reopens the keychain; a macOS-only method on every extractor's input. |
| a password function in the `masterkey.Locator` | a second path for the same secret, beside the extractor input.                                                                         |
| `keychain.Login`                               | collides with `record.Login`, a saved login.                                                                                           |
| `masterkey.Keychain`                           | `masterkey` holds Chromium keys; Safari's saved logins are not keys.                                                                   |

### 9. Leaf helpers — `timeconv` and `sqlitex`

**`timeconv`: one function per stored encoding, named for what the caller holds.** `Chromium` reads `base::Time`, microseconds since 1601; `CoreData` reads `CFAbsoluteTime`, seconds since 2001; `UnixMicro`, `UnixMilli`, and `Unix` read the three Unix units Firefox stores, named after the `time` functions they wrap. Not `WebKit`: in this tool WebKit is Safari's engine, and Safari's timestamps are Core Data's.

**`sqlitex` exports `CountRows` and `QueryRows` only.** The row-callback helper behind them, v1.1.0's `sqliteutil.QuerySQLite`, has no other caller, and as an export it would read `sqlitex.QuerySQLite`.

## Summary table

| Concept                 | Decision                                                                | Rejected                                                                                                         |
| ----------------------- | ----------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------- |
| Supported-browser table | `browser.Supported()` (this OS) + `browser.Lookup` (whole table)        | `catalog.All()`, `registry.All()`, `browser.Known()`                                                             |
| Per-record source       | flat `Browser` + `Profile`, set by `record.Stamp`                       | `Owner`, `Origin`, `Provenance`, `Source` types                                                                  |
| Extractor input         | `browser.ProfileSnapshot{Path, MasterKeys}`                             | `browser.Input`, `browser.Secrets`, `masterkey.Lazy`, `masterkey.Keys{Set}`                                      |
| macOS login keychain    | `keychain.Keychain`: one per run, opened on first use                   | `ProfileSnapshot.KeychainPassword`, a password function in the `Locator`, `keychain.Login`, `masterkey.Keychain` |
| Browser base            | `Base` (Chromium / Firefox / Safari)                                    | `Family`, `Engine`, `Platform`                                                                                   |
| Browser product         | `Def.ID` (no enum); format overrides keyed by ID                        | `Variant` / `Brand` enum                                                                                         |
| Data categories         | `category.Category` + bare `category.Cookie`…; zero is "none"           | `record.Kind`, `CategoryCookie`…, `model`                                                                        |
| Records                 | `record.Login`, `record.Cookie`…; sealed `record.Record`                | `record.CookieItem`, `record.Password`, `record.Item`                                                            |
| Data directory          | `browser.UserDataDir`, identified by a `DirID`                          | `Installation`                                                                                                   |
| Extractor output        | `browser.Collector` (`Add` + `Fail`)                                    | `browser.Sink`, `Emit`, an `Emit` func, `Output`, `Recorder`                                                     |
| Key retrieval input     | `masterkey.Locator`, built by `chromium.KeyLocator`                     | `masterkey.RetrieveInput`, `Hints`                                                                               |
| Pipeline values         | `Discovery`, `Plan`, `Summary`                                          | `type Run`                                                                                                       |
| Unit of work            | `app.Extraction{DirID, Profile, Category}`                              | `Job`, `Task`, `Target`, `Unit`                                                                                  |
| Snapshot                | `snapshot.Dir` + `snapshot.Snapshot`, returned by `Dir.Snapshot(dirID)` | `snapshot.FS`, `snapshot.Tree`, `snapshot.Copy`, `snapshot.Files`, `Dir.CopyOf`, `type Dir string`               |
| Windows packages        | `winapi`, `abe`                                                         | `platform/windows`                                                                                               |
| Leaf helpers            | `timeconv.Chromium`…`CoreData`; `sqlitex.CountRows`, `QueryRows`        | `timeconv.WebKit`, an exported `sqlitex.QuerySQLite`                                                             |

The only cross-row couplings: the `category` package and the bare record names are one decision, and the flat `Browser`/`Profile` source replaces every mention of `Origin`.
