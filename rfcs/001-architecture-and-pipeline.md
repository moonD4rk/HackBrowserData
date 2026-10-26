# RFC-001: Architecture & Pipeline

## The shape of a run

A run is a one-way pipeline of seven stages. Each stage takes the previous stage's output and produces the next stage's input. No stage reaches backward. The orchestration lives in `internal/app`; the stages themselves are implemented by the packages named below.

```
            ┌───────────┐
  argv ───▶ │ Validate  │  flags → app.Request; no browser files touched, nothing created
            └─────┬─────┘
                  ▼
            ┌───────────┐
            │ Discover  │  which user data dirs and profiles exist on this machine
            └─────┬─────┘
                  ▼
            ┌───────────┐
            │   Plan    │  user data dirs × profiles × requested categories
            └─────┬─────┘  → Extractions (+ Skipped, with reasons) + the union of files
                  ▼
   ┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄  Execute, once per user data dir  ┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄
            ┌───────────┐
            │  Capture  │  copy the user data dir's planned files into a temporary snapshot
            └─────┬─────┘
                  ▼
            ┌───────────┐
            │  Extract  │  parse snapshot files → records; secrets fetched lazily
            └─────┬─────┘  (first decrypt in a key scope triggers retrieval)
                  ▼
            ┌───────────┐
            │  Export   │  append each extraction's records to today's files,
            └─────┬─────┘  then remove the snapshot and take the next user data dir
   ┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄
                  ▼
            ┌───────────┐
            │  Report   │  every result → summary on stderr
            └───────────┘
```

## Stage contracts

### 1. Validate

Turns parsed flags into a typed `app.Request`: which categories, which browser, the `-p` override, output directory, format, `--zip`, and how the keychain password is supplied. Rejects bad input here — an unknown format, category, or browser ID, or an output directory that cannot be created, fails now with exit status `1` (today's status), **before** any browser is discovered and **before** any keychain prompt. Today `-c` and `-f` are checked only after discovery and the prompt; that ordering bug is designed out here.

Validate touches no browser files and creates nothing: the output-directory check only confirms that `-d` is a directory or that its nearest existing ancestor is a writable directory. For `restore`, reading `--keys` is part of Validate, because `-b` there is checked against the dump's vaults ([RFC-009](009-cli-surface.md)). `-p` combined with `-b all` logs a warning and is ignored, as today, instead of being dropped silently.

**In:** parsed flags. **Out:** a validated `app.Request`. **Owner:** `cmd/hack-browser-data` (flag wiring) + `app.Validate` (rules).

### 2. Discover

Enumerates which supported browsers are present and which profiles each one has, on this OS. Produces `browser.UserDataDir` values — one per data directory found, so two Arc packages are two user data dirs — each holding its `Profile` list. Discovery is read-only metadata work: it stats files and directories; it does not open databases or touch secrets. The one exception is Safari, which lists its named profiles only inside `SafariTabs.db`: its Discover captures that single file into a temporary snapshot and reads the copy, never the live database ([RFC-005](005-browser-definitions.md)). A user data dir that exists but cannot be read is not dropped: it comes back as an `error` result for that browser ([RFC-005](005-browser-definitions.md)).

**In:** `app.Request` + `browser.Supported()`, this OS's rows. **Out:** an `app.Discovery` — the user data dirs found, plus the failures. **Owner:** `app.Discover`, which dispatches on `Def.Base` to `internal/browser/{chromium,firefox,safari}`.

### 3. Plan

The cross product. For each user data dir, each profile, and each requested category, the profile's `Extractors` say whether the category is supported and which source files it needs. Plan turns that into a flat list of `Extraction`s, a `Skipped` list with a reason for every requested pair that can't run (`unsupported` by this browser, `absent` on disk) plus Discover's failures, and the union of files to capture, deduplicated by user data dir and path — so a file two categories share (Chromium `History` feeds both history and downloads) is copied once, where today it is copied twice. Plan only stats files; it opens nothing. A `Plan` is plain data, with no functions inside, so `list` prints it and tests compare it.

**In:** an `app.Discovery` + `app.Request`. **Out:** `app.Plan{Dirs, Files, Extractions, Skipped}`. **Owner:** `app.NewPlan`.

### 4–6. Capture, Extract, Export — once per user data dir

`app.Execute` runs the next three stages one user data dir at a time. It captures the dir's files into a temporary snapshot, runs the dir's extractions — each extraction's records go to Export as it finishes — and removes the snapshot before it captures the next dir. At most one user data dir's copies of `Login Data`, `Cookies`, and the like are on disk at once, close to today's per-profile session; a run-long snapshot directory would keep every browser's copies until the end, and peak disk use would be all of them together. `archive` is the one exception: it captures every user data dir into one snapshot directory, because the zip of that directory is what it produces ([RFC-007](007-snapshot-and-acquisition.md)).

#### 4. Capture

Copies the user data dir's planned files into its snapshot, a subdirectory named by the dir's ID ([RFC-007](007-snapshot-and-acquisition.md)). After Capture, the live browser profile is never read again. Capture owns the awkward realities of reading files a running browser holds open — the Windows locked-file fallback, SQLite `-wal`/`-shm` sidecars, LevelDB lock files — so every parser downstream reads plain, stable files. Capture keeps going when a file fails: a file that vanished since Plan turns its extraction into `absent`, one that failed to copy turns its extraction into `error`, and every other extraction runs.

**In:** the user data dir's share of `Plan.Files`. **Out:** the dir's `snapshot.Snapshot`, inside a `snapshot.Dir`. **Owner:** `internal/snapshot`.

#### 5. Extract

For each extraction, open its source files _from the snapshot_ and parse them into records. Secrets are lazy: each is retrieved once per key scope, by the first record that needs it — the user data dir for Chromium (`ProfileSnapshot.MasterKeys`), the profile for Firefox, the run for the macOS login keychain ([RFC-006](006-key-retrieval-and-crypto.md)); later extractions in the same scope reuse it. A run that only asked for bookmarks and history never retrieves a key, never opens the keychain, and never prompts.

A record whose secret can't be recovered is still emitted with that field empty (today's output) and reported as a record-level problem, which makes the extraction `partial`.

**In:** the user data dir's extractions + its snapshot. **Out:** records and record-level problems, through a `browser.Collector`. **Owner:** `internal/browser/*` extractors + `internal/masterkey` + `internal/keychain` + `internal/crypto`.

#### 6. Export

Appends each extraction's records, as it finishes, to today's files in today's formats ([RFC-008](008-output-and-export.md)): one file per non-empty category, field names frozen, rows in a deterministic order defined once per category.

**In:** records + `app.Request`. **Out:** files on disk (and the `--zip` archive). **Owner:** `internal/export`.

### 7. Report

Collects every result — each extraction's, and each failure that concerns a whole browser or user data dir — and prints a summary on stderr. The summary is additive: it is not part of the compatibility contract, and it never changes the exit status.

**In:** an `app.Summary`. **Out:** the summary. **Owner:** `app.Report`.

## The outcome model

Every extraction resolves to exactly one outcome. This is the single most important quality lever in the design: it replaces "an error was logged somewhere" with "the run completed and told you precisely what it got."

| Outcome       | Meaning                                                                                                                                | Example                                  |
| ------------- | -------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------- |
| `ok`          | Ran with no record-level problems. A source with no rows is `ok` with zero records, not `absent`.                                      | 224 cookies decrypted.                   |
| `partial`     | Ran and emitted records, but reported record-level problems — a secret that could not be recovered, a row that could not be parsed.    | 224 cookies, 4 with an empty value.      |
| `absent`      | Supported, but a required source wasn't on disk, or none of its optional ones was.                                                     | Browser installed, no `Login Data` file. |
| `unsupported` | This browser can't produce this category.                                                                                              | Firefox credit cards.                    |
| `error`       | Failed as a whole: the extractor returned an error, a source failed to copy, or problems were reported and no record could be emitted. | Snapshot copy failed.                    |

`partial` is first-class, not an afterthought: a few undecryptable rows are normal in the field and must not look like total failure.

A failure that concerns a whole browser or user data dir rather than one extraction — Discover could not read it, key retrieval failed — is reported once, as an `error` result that names only the user data dir. The extractions it touches still get their own outcomes: after a failed key retrieval, passwords and cookies are still emitted with their encrypted fields empty, so those extractions end `partial`.

## Exit status (compatibility contract)

| Status | Meaning                                                                                                      |
| ------ | ------------------------------------------------------------------------------------------------------------ |
| `0`    | The run completed. Every result — including `partial`, `error`, and "no browsers found" — is in the summary. |
| `1`    | The run could not start or finish: a usage error, an unreadable `--keys`, an output write failure.           |

This is today's behavior and stays frozen. `absent` and `unsupported` are not failures — asking all browsers for all categories and getting honest "not here" answers is a successful run. Finer exit codes (separate usage and partial statuses) are DEFERRED; if ever added, they must be opt-in.

## Why a pipeline and not the current interface

Today each browser package re-implements the orchestration: Chromium, Firefox, and Safari each copy files into a session, switch on the requested categories, and assemble results inside `Extract`, and per-fork special cases (`p.kind == types.ChromiumYandex`) sit inside the Chromium switch. Bugs live where nothing else checks them: Firefox sorts history ascending while the other browsers sort descending, and the archive keys files by browser key alone (`browser/archive.go`, `WriteArchive`), so the second Arc user data dir is dropped.

Splitting the run into named stages means:

- Each concern is implemented and tested **once**, not per browser.
- A browser contributes only what is browser-specific: where its files are (Discover) and how to parse them (Extract). Everything else is shared machinery.
- The `Plan` is a value you can print, which is all `list` is.
- Failures are localized to an extraction and reported, never fatal to the run.
