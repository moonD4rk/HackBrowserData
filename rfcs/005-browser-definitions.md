# RFC-005: Browser Definitions

## The table

HBD knows a fixed set of browsers. Each is a `browser.Def`: a static description of how the browser is classified and how it appears on each OS. The table is data, not code — adding a plain fork is adding a row.

```go
type Def struct {
    ID      string    // CLI -b value and keys.json vault key; also the first user data dir's DirID: "chrome"
    Name    string    // display name, stamped on every record: "Chrome"
    Base    Base      // Chromium | Firefox | Safari — the only classification enum (RFC-004)

    Darwin  Platform  // no UserDataDirs ⇒ not supported on that OS
    Linux   Platform
    Windows Platform
}

type Base int

const (
    Chromium Base = iota + 1
    Firefox
    Safari
)

// Platform is how one browser appears on one OS.
type Platform struct {
    UserDataDirs  []string           // relative to the home dir; globs allowed; every match is its own UserDataDir
    KeychainLabel string             // macOS Keychain account / Linux Secret Service label; "" = none
    Exe           string             // Windows: the browser program's executable, used by ABE ("chrome.exe")
    ExeFallbacks  []string           // Windows: program paths tried when the registry has no App Paths entry
    ABE           masterkey.ABEKind  // Windows: App-Bound Encryption dispatch; ABENone = DPAPI only
}
```

A row, for scale:

```go
{
    ID: "chrome", Name: "Chrome", Base: Chromium,
    Darwin: Platform{UserDataDirs: []string{"Library/Application Support/Google/Chrome"}, KeychainLabel: "Chrome"},
    Linux:  Platform{UserDataDirs: []string{".config/google-chrome"}, KeychainLabel: "Chrome Safe Storage"},
    Windows: Platform{
        UserDataDirs: []string{"AppData/Local/Google/Chrome/User Data"},
        Exe:          "chrome.exe",
        ExeFallbacks: []string{`%ProgramFiles%\Google\Chrome\Application\chrome.exe`, …},
        ABE:          masterkey.ABEChromeBase,
    },
},
```

The per-OS fields are what v1.1.0's code spread across three places: the `KeychainLabel` of each `types.BrowserConfig`, the `WindowsABE` flag, and the separate `winutil.Table` entry joined to it by key. Each fork's label differs per OS (Edge uses `Microsoft Edge` on macOS but `Chromium Safe Storage` on Linux), which is why the label lives in `Platform`, not on `Def`. `UserDataDirs` names data directories; `Exe` and `ExeFallbacks` name the program installation — two different things, kept apart in name as well ([RFC-004](004-naming-decisions.md)).

One table serves every OS. `Def.Platform()` returns the running OS's entry (a build-tagged one-liner). `Supported()` returns the rows whose entry has `UserDataDirs` — the per-OS list the help text shows, and exactly what `-b` accepts. `Lookup` searches the whole table, so `restore` resolves a vault's `Def` whatever the host OS and a Windows-only fork restores on macOS ([RFC-007](007-snapshot-and-acquisition.md)).

Two axes, kept apart ([RFC-004](004-naming-decisions.md)). `Base` is the only enum and answers "what is this built on" — it selects the base package and the key path. Product/brand identity is just `ID` (a string), so a new fork is a new row, not a new enum constant. Release channel (stable/beta/dev/canary) gets no field, because nothing dispatches on it: each installed channel that keeps its own user data dir is its own `Def`, with an `ID` that names product and channel together (`chrome-beta`).

Accessors (names per [RFC-004](004-naming-decisions.md)):

```go
browser.Supported() []Def              // this OS's rows, in table order
browser.Lookup(id string) (Def, bool)  // any row, whatever the OS
browser.IDs() []string                 // this OS's IDs, sorted, for -b's help and errors
```

## How a browser produces data

`Def` says _where_. `Base` says _how_. Each base package supplies two functions:

```go
// one per base, in internal/browser/{chromium,firefox,safari}
func Discover(def browser.Def, opts browser.DiscoverOptions) ([]browser.UserDataDir, error)
func Extractors(dir browser.UserDataDir, p browser.Profile) browser.Extractors
```

`app` calls them through a `switch def.Base`. The dispatch cannot live in `browser`: the base packages import `browser` for these very types, so `browser` calling them would be an import cycle, and `gochecknoinits` rules out init-time registration. These are plain functions, not an interface named `Engine` — that word is rejected project-wide.

The Chromium base supplies two more, for the files no single category owns (below):

```go
// internal/browser/chromium only
func KeyLocator(dir browser.UserDataDir, snap snapshot.Snapshot) (masterkey.Locator, error)
func ArchiveSources(dir browser.UserDataDir) []browser.Source
```

Safari's `Extractors` takes one more argument, the run's login keychain, which its password extractor captures. Only the default profile's map gets a password extractor, because the keychain belongs to the user, not to one profile ([RFC-006](006-key-retrieval-and-crypto.md)). `LoginKeychain` is the one-method interface Safari declares; `*keychain.Keychain` satisfies it.

```go
// internal/browser/safari only
func Extractors(dir browser.UserDataDir, p browser.Profile, kc LoginKeychain) browser.Extractors
```

## Discovery

Discovery answers one question — which user data dirs and profiles exist — and nothing else. Its rules:

- **Explicit inputs.** Everything comes in through `DiscoverOptions`. `app` resolves the home directory once; if that fails the run stops with status `1`. v1.1.0 resolved it in a package-level variable with the error discarded, so a failure silently turned every path into a wrong absolute one, and tests could not substitute a home directory.
- **Every match counts.** Every `UserDataDirs` entry is expanded, globs included, and **every** existing match becomes its own `UserDataDir` — as in v1.1.0, which is what keeps the user data dirs of both of a machine's Arc MSIX packages. The first one of a `Def` gets the `DirID` `Def.ID`, later ones `-2`, `-3`… in path order, so IDs are stable from run to run.
- **Errors are not absence.** A user data dir that does not exist yields nothing. Any other failure — permission denied, a malformed glob — is returned (joined across entries, next to whatever was found) and becomes an `error` result for that browser in the summary. In v1.1.0 both the Chromium and the Firefox profile scans returned nothing on any read error, and glob errors were dropped while an unmatched pattern was kept as a literal path, so "unreadable" looked exactly like "not installed".
- **Profiles by v1.1.0's rules.** Chromium directories holding `Preferences` or `Preferences_02` (Tencent forks), with flat-layout and copied-tree fallbacks; Firefox directories holding known data files; Safari's default profile plus its named profiles (macOS 14+). Whatever the base, a profile counts only when it holds a file some category reads, so a profile created and never used is not listed, archived, or keyed. The named profiles are listed only inside `SafariTabs.db`, so Safari's Discover captures that one file into a temporary snapshot with the snapshot copier and reads the copy, never the live database.
- **No source checks.** Whether a category's file exists is Plan's question, answered as `absent`, or as `error` when the file cannot be checked. v1.1.0 checked every source while constructing the browser and silently dropped a profile that had none; `list` keeps that display rule (profiles with at least one source), but the decision lives outside discovery.
- **`-p` replaces `UserDataDirs` for one `Def`.** When `DiscoverOptions.ProfilePath` is set, discovery reads that path instead of the table's entries, and each base interprets it by the rules in [RFC-009](009-cli-surface.md).

## Per-fork overrides — data stays data

Most Chromium forks are pure rows. A few store data their own way; v1.1.0 gave them their own `types.BrowserKind` values with `if kind == …` checks across files:

| Fork IDs                      | What differs                                                                                                       | Override                                        |
| ----------------------------- | ------------------------------------------------------------------------------------------------------------------ | ----------------------------------------------- |
| `yandex`                      | `Ya Passman Data` and `Ya Credit Cards`, with Yandex's own key wrapping and row format; no account-store passwords | password and credit-card sources and extractors |
| `opera`, `opera-gx`, `vought` | extensions under `extensions.opsettings`                                                                           | extension extractor                             |

These overrides live inside `internal/browser/chromium`, in one unexported table keyed by `Def.ID` — never by the `DirID`, which differs for a second user data dir — applied when `Extractors` builds a profile's map. No enum is involved and the browser table stays free of code. A unit test asserts that every override key resolves through `Lookup` to a Chromium `Def`, so a typo cannot silently drop an override.

`keys.json` keeps writing v1.1.0's `kind` values (`chromium`, `chromium-yandex`, `chromium-opera`), derived from the same table, so v1.1.0 can still read a new dump. When reading, `restore` resolves the `Def` from the vault's `browser` ID and falls back to `kind` only for an ID this build doesn't know.

## Shared and per-profile files

Most sources belong to one profile (cookies, history) and are declared relative to the profile directory. A few belong to the whole user data dir — `Local State` above all — and are marked `Shared`, relative to the user data dir itself ([RFC-003](003-core-types.md)). Plan resolves both into paths under the user data dir and captures the union once per user data dir, so `Local State` is copied once however many profiles and categories need it. The flag is explicit on purpose: rewriting every source path per profile while remembering to leave the shared ones alone would make correctness depend on the order of two builder calls.

Two Chromium facts concern a whole user data dir rather than one category, and both stay in the Chromium base, so `app` never spells a Chromium file name:

- **Key material.** The extractors that decrypt — passwords, cookies, credit cards — list `Local State` as a `Shared` source, so it is captured only when a decrypting category is requested. `KeyLocator` builds the dir's `masterkey.Locator` from the captured `Local State` and the `Platform` fields; on macOS `app` adds the run's login keychain, then wraps retrieval in the dir's call-once, which every `ProfileSnapshot` of the dir shares as its `MasterKeys` ([RFC-006](006-key-retrieval-and-crypto.md)).
- **Layout markers.** `archive` also captures `Local State` and each profile's `Preferences` (or `Preferences_02`) marker, so a reader without the manifest can still find every profile; `ArchiveSources` names them.

## Adding a browser (the whole motion)

For a new Chromium fork:

1. **Add a row** to the table in `internal/browser`: `ID`, `Name`, `Base: Chromium`, and a `Platform` per OS (user data dirs, keychain label; on Windows `Exe`, `ExeFallbacks`, `ABE`).
2. **If `ABE` is not `ABENone`**, add the fork's row to the payload's `com_iid.c` — the CLSID/IIDs and vtable slot the injected payload needs. The technique and how to mine those identifiers are in [RFC-016](016-chrome-abe-integration.md).
3. **Only for a fork with its own on-disk format**, add an override in the Chromium package.
4. **Nothing else.** The fork inherits the entire Chromium base: discovery, every extractor, the key path, snapshot, export, reporting.

For a genuinely new base (not Chromium/Firefox/Safari), you add a new `internal/browser/<base>` package implementing `Discover` + `Extractors`, a new `Base` value, and a case in `app`'s switch. This is rare — the three bases cover the field.

## Consistency, enforced by tests

Folding `winutil.Table` into `Platform` removes the Go-side join, and with it the need for v1.1.0's `TestWinUtilTableCoversABEBrowsers`: one struct cannot disagree with itself. What remains is the C side — the payload runs inside the browser process with no Go runtime, so its table stays in `com_iid.c`. A Go test inside `browser`, over the whole table, parses that file and fails when a Windows entry whose `ABE` is not `ABENone` has no row there with the same executable and ABE kind; the C table may list browsers the Go table does not, such as Avast ([RFC-011](011-testing-and-quality.md)). A mismatch fails `go test`, not production.

## Why a table beats per-browser types

The current design has per-OS `platformBrowsers()` lists of `BrowserConfig`, a `WindowsABE` flag, a Windows table joined by key, and two Chromium kinds switched on across files. The table collapses that to rows + three base packages + a short override table. The payoff:

- The set of supported browsers is **legible in one place** — good for the README, good for a talk slide, good for auditing coverage across all three OSes at once.
- A plain fork is a data change, reviewable at a glance.
- Base behavior is implemented three times total, not once per browser.
