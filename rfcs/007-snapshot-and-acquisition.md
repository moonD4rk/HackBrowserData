# RFC-007: Snapshot & Acquisition

## The snapshot-first rule

No parser ever reads a live browser profile. Capture copies the files a plan needs into a **snapshot directory** — one snapshot per user data dir — and every extractor reads only from its user data dir's snapshot, through the `ProfileSnapshot` that wraps it. Today's code already does this per profile (`filemanager.Session` copies sources into a temp directory before parsing); the rewrite makes it one stage with one contract, so:

- A running browser holds files open and mutates them mid-read. The snapshot is a point-in-time copy; parsers see stable bytes.
- Locked-file handling (a Windows reality) stays in one place — the snapshot's copier — and every acquisition path uses it.
- `archive` and `restore` are not special modes: `archive` is "capture, then zip the snapshot directory", `restore` is "unzip or open a snapshot directory, then extract from it". Same code path.

## Contract

```go
type File struct {
    DirID string // the user data dir's ID; also the snapshot it lands in
    From  string // absolute path of the user data dir it is copied from
    Rel   string // slash-separated path relative to From, and to the snapshot
    IsDir bool
}

type Dir struct{ /* unexported */ }

func Temp() (*Dir, error)            // a new temporary snapshot directory, mode 0700
func Open(path string) (*Dir, error) // an existing one, used in place
func Unzip(src string) (*Dir, error) // into a temporary snapshot directory

func (d *Dir) Capture(ctx context.Context, files []File) error
func (d *Dir) Snapshot(dirID string) Snapshot // one user data dir's snapshot
func (d *Dir) Zip(dst string) error           // archive transport
func (d *Dir) Close() error

type Snapshot struct{ /* unexported */ }

func (s Snapshot) Path(rel string) (string, error) // absolute path inside the snapshot
```

`Snapshot` returns real paths, not readers: `modernc.org/sqlite` and `goleveldb` open files by path, and SQLite reads its `-wal` file from beside the database. A zip can't serve either, so the snapshot is always a directory and zip is only how it travels — today's `restore` already unzips into a temp directory first, and that stays.

**Errors.** `Capture` keeps going when one file fails; it returns an error only when it cannot go on — a cancelled context, an unwritable snapshot directory. A file that vanished before Capture reached it reads back from `Snapshot.Path` as `fs.ErrNotExist`, which makes its extraction `absent`; a file whose copy failed reads back as that copy's error, which makes its extraction `error`. Every other extraction runs.

## Lifetime

`dump` captures one user data dir at a time ([RFC-001](001-architecture-and-pipeline.md)): `Execute` creates a `Temp` snapshot directory for the dir, captures its files, runs its extractions, and `Close`s it before the next dir. At most one user data dir's copies of `Login Data`, `Cookies`, and the like exist at once — close to today's per-profile session, and far from a run-long snapshot directory holding every browser's copies until the end. `archive` captures every user data dir into one snapshot directory, because the zip of that directory is its product. `restore --data-zip` unzips into a `Temp` snapshot directory; `restore --data-dir` `Open`s the given directory, which `Close` leaves in place.

Because it holds those copies, a temporary snapshot directory is created with mode `0700` and always removed — on success, on error, and on cancellation (the run's context is cancelled on Ctrl-C, `SIGTERM`, or `SIGHUP`; [RFC-009](009-cli-surface.md)). The live profile still exists, so removing the copy never loses anything.

## Layout and user data dir identity

A snapshot directory is `<dir>/<user-data-dir-id>/<path relative to the user data dir>`: one subdirectory, one snapshot, per user data dir. For the first user data dir of a browser the ID is its `Def.ID`, so the layout is exactly today's archive layout (`<browser-key>/<User Data layout>`) and v1.1.0's `restore` reads new archives unchanged. A second one of the same browser — the second of Arc's two MSIX packages on Windows — gets `arc-2/`, where today's archive keys every file by browser key alone and silently drops it.

An archive captures what today's does — `Local State` and each profile's `Preferences` marker (both named by the Chromium base, [RFC-005](005-browser-definitions.md)), the requested categories' sources, and the account-synced `Login Data For Account` — so a reader without the manifest can still find every profile.

`keys.json` joins to the archive by the vault's `browser` key, as today. A vault for a second user data dir carries an extra optional `id` field (`"arc-2"`) — the `DirID` on the wire, per the wire-name rule; readers that don't know the field, including v1.1.0, ignore it and restore the first one, as they do today. The dump version stays `"2"`.

## Manifest

A new snapshot directory carries `manifest.json` at its root: a schema version, and per user data dir its ID, `Def.ID`, `Base`, profiles, and captured paths. `restore` uses it, when present, to rebuild user data dirs and profiles without re-discovering anything — that is its `Discovery` ([RFC-003](003-core-types.md)) — and then runs Extract → Export → Report exactly as a live run would. Restore trusts no manifest path that would leave its own directory of the archive. Archives without a manifest — every archive made before the rewrite — still restore through today's path: the keys.json vaults plus the directory layout. v1.1.0 ignores the extra file.

## Copy semantics

- Capture only ever **reads** the browser's files; it never deletes or moves anything in a real profile.
- A file is copied together with its SQLite `-wal` and `-shm` sidecars when they exist; a directory (LevelDB, Session Storage) is copied whole, skipping its `LOCK` file — both as today.
- Chromium databases are opened as copied, `-wal` included; Firefox and Safari databases are opened with `journal_mode=off` — both as today.
- `Zip` writes to a temp file beside the target and renames it into place only after the zip is closed; a failed archive leaves no partial file behind.
- `Unzip` rejects any entry whose path would land outside the snapshot directory.

The `--zip` output-packing bug (originals deleted before the zip is written) is an Export concern and is fixed there ([RFC-008](008-output-and-export.md)).

## Locked-file handling

Chromium on Windows has held `Cookies` under an exclusive lock since Chrome 114, so a plain copy fails while the browser runs; on macOS and Linux file locks are advisory and never block a reader. Reading such a file is Windows-specific and lives in `winapi` (the technique itself is documented in [RFC-021](021-windows-locked-file-bypass.md)). The snapshot's copier tries a plain copy first and, on Windows only, falls back to the locked-file copy when the plain copy fails — today's order. macOS and Linux use the plain copy. Extractors are unaware of which strategy ran.

## Scope

`archive`, `dumpkeys`, and `restore` stay Chromium-only, as today: Firefox and Safari have no portable key in `keys.json`. Archiving them is DEFERRED; it would add capability, which this rewrite does not.

## Why this is worth a dedicated stage

Acquisition is where the messiest real-world facts live: files that move, lock, or vanish between discovery and read. Isolating all of that in Capture, behind a read-only `Snapshot`, means the parsers downstream are pure file-in/record-out functions. That is the difference between a parser you can unit-test with a fixture file and one that needs a live browser to exercise.
