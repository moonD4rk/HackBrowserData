# RFC-008: File Acquisition & Platform Quirks

**Author**: moonD4rk
**Status**: Living Document
**Created**: 2026-04-05

## 1. Overview

Browsers keep their data files open and often locked while running. Chromium on Windows is particularly aggressive: it holds exclusive locks on databases like `Cookies` via `PRAGMA locking_mode=EXCLUSIVE`. Even on macOS and Linux, reading directly from a live database can produce corrupt or inconsistent results.

The solution is a **copy-then-read** strategy: copy all needed files to an isolated temporary directory, then extract data from the copies. The `filemanager` package manages this lifecycle, while platform-specific fallbacks handle locked files on Windows.

## 2. Session Management

A `Session` wraps a single temporary directory for one browser profile extraction run:

1. **Create** — `NewSession()` creates a unique temp directory via `os.MkdirTemp("", "hbd-*")`
2. **Acquire** — `Acquire(src, dst, isDir)` copies a browser file or directory into the session
3. **Cleanup** — removes the entire temp directory tree, always called with `defer`

## 3. Acquire Flow

`Acquire` is the single entry point for copying browser files:

```
Acquire(src, dst, isDir)
  ├── isDir=true  → copyDir(src, dst, skip="lock")
  │
  └── isDir=false → copyFile(src, dst)
                      ├── success → copy -wal and -shm companions if present
                      └── failure + Windows → copyLocked(src, dst) fallback
```

### SQLite Companion Files

SQLite databases using WAL mode maintain `-wal` (write-ahead log) and `-shm` (shared memory) files. After a successful file copy, `Acquire` automatically copies these companions if they exist. Without the WAL file, recently written data (cookies set in the last few seconds) would be missing.

## 4. File Deduplication

Multiple categories can share the same source file:

| Engine | Categories | Shared Source |
|--------|-----------|---------------|
| Chromium | History + Download | `History` |
| Firefox | History + Download + Bookmark | `places.sqlite` |

Each category gets its own destination path in the temp directory, so the same source file may be copied multiple times. This is intentional — each extract function expects its own independent file path, and the copy cost is negligible for small SQLite files.

## 5. Windows Locked File Handling

Chromium on Windows holds exclusive locks on certain databases (notably `Cookies`), causing standard file reads to fail. Chrome introduced `PRAGMA locking_mode=EXCLUSIVE` for the cookies database starting from Chrome 114 (2023) via the "Lock profile cookie files on disk" feature, preventing external processes from reading cookie data while the browser is running. This is a Windows-specific problem — macOS and Linux use `fcntl`/`flock` advisory locks that do not prevent reading by other processes.

A dedicated technique using Windows kernel APIs (DuplicateHandle + memory-mapped I/O) is used to bypass these locks. See [RFC-009](009-windows-locked-file-bypass.md) for the full technical details.

## 6. LevelDB Directory Handling

Chromium stores localStorage and sessionStorage as LevelDB directories:

| Category | Path | Type |
|----------|------|------|
| LocalStorage | `Local Storage/leveldb/` | directory |
| SessionStorage | `Session Storage/` | directory |

When `isDir=true`, `Acquire` copies the entire directory while **skipping the `LOCK` file**. LevelDB uses this file for single-process access control; copying it could interfere with the running browser.

## 7. SQLite Query Helpers

### QuerySQLite

Encapsulates the common SQLite extraction pattern: validate file exists → open database → optional `PRAGMA journal_mode=off` → execute query → iterate rows with error-tolerant scan callback.

Row-level scan errors are logged and skipped (graceful degradation for corrupt records), while database-level errors abort the query.

### QueryRows[T]

A generic wrapper (Go 1.18+) that collects results into a typed slice, eliminating boilerplate. Each extract function only needs to provide the scan function.

### Firefox journal_mode=off

All Firefox extract calls use `journal_mode=off`. Firefox databases use WAL mode in production, and the `modernc.org/sqlite` driver may attempt WAL replay on a temp copy. Disabling the journal prevents this and treats the database as a read-only snapshot.

Chromium extract calls do **not** disable journal mode because `Acquire` already copies the WAL/SHM companions, giving SQLite everything it needs for a clean WAL replay.

## 8. File Utilities

- **CompressDir** — compresses all files in the output directory into a single `.zip` file (used by `--zip` flag). Original files are removed after archiving.

## Related RFCs

| RFC | Topic |
|-----|-------|
| [RFC-002](002-chromium-data-storage.md) | Chromium data file locations |
| [RFC-004](004-firefox-data-storage.md) | Firefox data file locations |
| [RFC-009](009-windows-locked-file-bypass.md) | Windows locked file bypass technique |
