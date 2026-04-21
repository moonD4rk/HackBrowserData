# RFC-011: Safari Data Storage

**Author**: moonD4rk
**Status**: Living Document
**Created**: 2026-04-21

## 1. Overview

Safari is **macOS-only** and sandboxed under App Sandbox. Most of Safari's user data lives inside `~/Library/Containers/com.apple.Safari/Data/Library/` (the container root) and requires **Full Disk Access (TCC)** for third-party processes to read. A few legacy files still reside at `~/Library/Safari/` for backwards compatibility.

Unlike Chromium and Firefox, Safari does **not** encrypt bookmarks, history, cookies, downloads, or localStorage — all are stored in plaintext on disk. Passwords are the only encrypted category and are delegated to the macOS login Keychain (see [RFC-006](006-key-retrieval-mechanisms.md) §7).

Safari 17 (September 2023) introduced **multi-profile support**. Profile discovery therefore has two layers: a synthetic "default" profile mapped to the pre-profile legacy paths, plus one or more named profiles enumerated from `SafariTabs.db`.

## 2. Profile Structure

Each `profileContext` (in `browser/safari/profiles.go`) tracks five fields:

| Field | Meaning |
|-------|---------|
| `name` | Human-readable profile name, disambiguated for duplicates |
| `uuidUpper` | UUID in uppercase (used by `Safari/Profiles/<UUID>/` directories) |
| `uuidLower` | UUID in lowercase (used by `WebKit/WebsiteDataStore/<uuid>/` directories) |
| `legacyHome` | `~/Library/Safari` |
| `container` | `~/Library/Containers/com.apple.Safari/Data/Library` |

Empty `uuidUpper` marks the synthetic default profile.

### 2.1 Profile Discovery

The default profile is always emitted first. Named profiles come from `SafariTabs.db`:

```sql
SELECT external_uuid, title FROM bookmarks
WHERE subtype = 2 AND external_uuid != 'DefaultProfile'
```

`DefaultProfile` is Safari's sentinel string for the pre-profile era; it is filtered out because it is already represented by the synthetic default.

If the DB cannot be opened (missing, permission-denied), Safari falls back to scanning `Safari/Profiles/` for any directory whose name is a canonical 8-4-4-4-12 UUID and synthesizing the name as `profile-<uuid[:8]>`. This makes profile discovery robust even when TCC blocks the SQL read.

Duplicate display names are disambiguated with `-2`, `-3`, … suffixes, deterministically by discovery order.

### 2.2 UUID Case Asymmetry

Safari uses two different casings for the same profile UUID across the container:

| Path prefix | Casing | Example |
|-------------|:------:|---------|
| `Safari/Profiles/<UUID>/` | Uppercase | `5604E6F5-02ED-4E40-8249-63DE7BC986C8` |
| `WebKit/WebsiteDataStore/<uuid>/` | Lowercase | `5604e6f5-02ed-4e40-8249-63de7bc986c8` |

`profileContext` stores both to avoid case-folding at every call site.

## 3. Data File Locations

### 3.1 Default Profile

| Category | Path | Format |
|----------|------|--------|
| History | `~/Library/Safari/History.db` | SQLite |
| Cookie | `Container/Cookies/Cookies.binarycookies`, then `~/Library/Cookies/Cookies.binarycookies` | BinaryCookies |
| Bookmark | `~/Library/Safari/Bookmarks.plist` | plist |
| Download | `~/Library/Safari/Downloads.plist` | plist |
| LocalStorage | `Container/WebKit/WebsiteData/Default/` | WebKit Origins dir |
| Password | macOS Keychain | — |

The Cookie path is resolved in priority order — the first candidate that exists wins. Modern (macOS 14+) installs keep cookies in the sandboxed container; the legacy path is kept as a fallback for upgraded systems.

### 3.2 Named Profiles

| Category | Path | Format |
|----------|------|--------|
| History | `Container/Safari/Profiles/<UUID>/History.db` | SQLite |
| Cookie | `Container/WebKit/WebsiteDataStore/<uuid>/Cookies/Cookies.binarycookies` | BinaryCookies |
| Download | `~/Library/Safari/Downloads.plist` (filtered by UUID) | plist |
| LocalStorage | `Container/WebKit/WebsiteDataStore/<uuid>/Origins/` | WebKit Origins dir |

Bookmark is intentionally **omitted** from named profiles: `Bookmarks.plist` is a shared plist with no per-entry profile tag, so it is attributed to the default profile only. Duplicate bookmarks would otherwise be emitted per profile.

Downloads is shared across all profiles but each entry carries a `DownloadEntryProfileUUIDStringKey`; the extractor filters at read time so each profile sees only its own downloads.

Passwords live in the user-scope Keychain, not on a per-profile basis — only the default profile emits passwords to avoid duplicates across the output.

## 4. Data Storage Formats

### 4.1 History (History.db — SQLite)

```sql
SELECT url, title, visit_count, visit_time
FROM history_items
LEFT JOIN history_visits ON history_items.id = history_visits.history_item
```

Schema notes:
- `visit_time` is a `REAL` column using the **Core Data epoch** (see Section 5)
- One item → many visits; the extractor takes the most recent visit per item
- Results are sorted by `visit_count` descending

### 4.2 Cookies (Cookies.binarycookies — binary)

Apple's proprietary BinaryCookies format — not SQLite, not a documented format. Parsed by the [go-binarycookies](https://github.com/moond4rk/go-binarycookies) library.

High-level layout:

```
| "cook" magic | page_count | page_sizes[]     | pages[]                  |
|--------------|------------|------------------|--------------------------|
| 4B           | 4B (BE)    | page_count × 4B  | variable                 |
```

Each page is an index-of-cookies table followed by per-cookie records. A cookie record carries flags (`isSecure`, `isHTTPOnly`), URL/name/path/value offsets into the record, and creation / expiry timestamps in Core Data epoch.

Cookie values are **plaintext** — no per-cookie encryption. This is a fundamental divergence from Chromium, which encrypts `encrypted_value` with the OS master key.

### 4.3 Bookmarks (Bookmarks.plist — property list)

A nested dictionary tree with a `WebBookmarkType` discriminator at each node:

| Type | Meaning | Additional keys |
|------|---------|-----------------|
| `WebBookmarkTypeList` | Folder | `Children` (array) |
| `WebBookmarkTypeLeaf` | URL entry | `URLString`, `URIDictionary.title` |

The extractor walks the tree recursively, collecting leaf nodes into a flat list. Folder names are not preserved (only URL + title pairs are exported).

### 4.4 Downloads (Downloads.plist — property list)

A flat structure with a `DownloadHistory` array. Relevant keys per entry:

| Key | Meaning |
|-----|---------|
| `DownloadEntryURL` | Source URL |
| `DownloadEntryPath` | Local filesystem path |
| `DownloadEntryBytesReceivedSoFar` | Bytes downloaded |
| `DownloadEntryProfileUUIDStringKey` | Owning profile's uppercase UUID, or `"DefaultProfile"` |

The extractor filters by the caller-provided owner UUID so each profile reports its own downloads. MIME type and start/end times are not stored by Safari — `MimeType` is always empty in the output.

### 4.5 Passwords (macOS Keychain)

Safari does **not** persist passwords to a file in its container. All credentials live in `login.keychain-db`, accessible via `InternetPassword` records. The extractor reads them directly through [keychainbreaker](https://github.com/moond4rk/keychainbreaker) and reconstructs the URL from `(protocol, server, port, path)`.

Default port handling:

| Protocol | Default port | URL rendering |
|----------|-------------:|---------------|
| `https` | 443 | `https://host/path` (port omitted) |
| `http` | 80 | `http://host/path` (port omitted) |
| `ftp` | 21 | `ftp://host/path` (port omitted) |
| Other | — | `scheme://host:port/path` |

The `htps` FourCC protocol code emitted by some Keychain entries is normalized to `https`.

Partial-extraction mode: if the Keychain cannot be unlocked (no `--keychain-pw` supplied, or the password is wrong), metadata-only records are still emitted — URL, username, timestamps — with `PlainPassword` left blank. See [RFC-006](006-key-retrieval-mechanisms.md) §7 for the full credential-extraction architecture.

### 4.6 LocalStorage (WebKit Origins — nested SQLite)

Safari 17+ stores localStorage under a **partition-aware nested tree**, rooted at:

| Profile | Root path |
|---------|-----------|
| Default | `Container/WebKit/WebsiteData/Default/` |
| Named | `Container/WebKit/WebsiteDataStore/<uuid>/Origins/` |

Under the root, two levels of hashed directories lead to the actual data:

```
<root>/<top-frame-hash>/<frame-hash>/
├── origin                         ← binary-serialized origins (top + frame)
└── LocalStorage/
    ├── localstorage.sqlite3       ← ItemTable(key TEXT UNIQUE, value BLOB NOT NULL)
    ├── localstorage.sqlite3-shm
    └── localstorage.sqlite3-wal
```

`top-frame-hash == frame-hash` for **first-party** storage. They differ for **partitioned third-party** storage (an iframe with a different origin than the top document). The named profile root additionally carries a `salt` sibling file used by WebKit's origin-hashing — skipped at traversal time.

The flat `WebsiteDataStore/<uuid>/LocalStorage/<scheme>_<host>_<port>.localstorage` layout used by older WebKit is **empty on modern Safari** and is not supported.

#### Origin file format

Two `origin` blocks back-to-back — top-frame then frame. Each block:

```
| scheme record            | host record              | port section    |
|--------------------------|--------------------------|-----------------|
| uint32_le len | enc byte | uint32_le len | enc byte | 0x00            |
| <len bytes>              | <len bytes>              |                 |
                                                        or
                                                       | 0x01 | uint16_le port |
```

- `enc byte`: `0x01` = Latin-1/ASCII (common), `0x00` = UTF-16 LE
- Port section: `0x00` marker means "use scheme default" (stored as port 0 in the parsed struct); `0x01` marker is followed by a 2-byte little-endian port

The extractor reads both blocks and reports the **frame origin URL** — that is what JavaScript's `window.localStorage` actually exposes in the partitioned case. If only the top-frame block is parseable, the extractor falls back to it.

#### ItemTable

```sql
SELECT key, value FROM ItemTable
```

Schema: `(key TEXT UNIQUE ON CONFLICT REPLACE, value BLOB NOT NULL ON CONFLICT FAIL)`.

Values are **UTF-16 LE** encoded JS strings. Oversized values (≥ 2048 bytes) are replaced with a size marker in the output — this matches the cap used by the Chromium extractor ([RFC-002](002-chromium-data-storage.md) §4.8) and keeps JSON/CSV exports bounded.

## 5. Time Formats

Safari uses the **Core Data epoch** — 2001-01-01 00:00:00 UTC, which is **978,307,200 seconds** after the Unix epoch. To convert a Core Data timestamp to Unix time, add `978307200` seconds.

| Data Type | Field | Storage |
|-----------|-------|---------|
| History | `visit_time` | REAL seconds, Core Data epoch |
| Cookies | `creation`, `expiry` | REAL seconds, Core Data epoch |
| Downloads | — | No timestamp stored |
| Passwords | Keychain `Created` | Already Unix time (via keychainbreaker) |
| LocalStorage | — | No timestamp stored |

Bookmarks carry no timestamp in Safari's plist representation.

## 6. Encryption

Safari's encryption story is deliberately thin:

| Category | Encryption |
|----------|------------|
| History | None (plaintext SQLite) |
| Cookies | None (plaintext binary format) |
| Bookmarks | None (plaintext plist) |
| Downloads | None (plaintext plist) |
| LocalStorage | None (plaintext SQLite; UTF-16 LE is an encoding, not encryption) |
| Passwords | macOS Keychain — see [RFC-006](006-key-retrieval-mechanisms.md) §7 |

The only encrypted category is passwords. Because they are not stored in Safari's own files at all, there is no Safari-specific cipher, key derivation, or master-key retrieval to document. See RFC-006 for the `InternetPassword` extraction path.

## 7. Platform Specifics

- **macOS-only**. There is no Safari on Windows or Linux.
- **Full Disk Access (TCC)** is required to read the sandboxed container. Without it, cookies / history / downloads / localStorage reads fail silently with permission errors at stat or open time. Legacy paths under `~/Library/Safari/` sometimes remain readable without FDA, but are mostly empty on modern systems.
- **Live-file safety** follows a live-vs-temp split:
  - **Live reads** (`SafariTabs.db` during profile discovery in `profiles.go`) use `?mode=ro&immutable=1`, which disables WAL replay and locking so the extractor cannot disturb a running Safari — it sees a consistent snapshot of the main DB as of read time, at the cost of missing any pending WAL content.
  - **Temp-copy reads** (`History.db`, `localstorage.sqlite3`, etc. via `filemanager.Session.Acquire`) use `?mode=ro` only. `Session.Acquire` copies the `-wal` / `-shm` sidecars alongside the main DB, so SQLite can replay uncommitted transactions on the copy — surfacing entries Safari has written to WAL but not yet checkpointed. Any `-shm` writes SQLite performs during replay land on the ephemeral copy and are deleted with the session.
- **Multi-profile availability**: requires Safari 17 (macOS 14 Sonoma) or newer. Older Safari versions have only the default profile; discovery degrades cleanly via the ReadDir fallback described in §2.1.
- **File acquisition**: all per-profile files are copied into a `filemanager.Session` temp directory before extraction, except the discovery-time `SafariTabs.db` read which opens the live file directly. See [RFC-008](008-file-acquisition-and-platform-quirks.md) for the general pattern.

## 8. Key Differences from Chromium and Firefox

| Aspect | Chromium | Firefox | Safari |
|--------|----------|---------|--------|
| Platform | Cross-platform | Cross-platform | **macOS-only** |
| Profile discovery | `Preferences` sentinel file | Any data file present | `SafariTabs.db` SQL + dir fallback |
| Profile naming | `Default`, `Profile 1`, … | `<prefix>.default-release` | Human-readable title from SafariTabs.db |
| Password storage | Encrypted SQLite (`Login Data`) | Encrypted JSON (`logins.json`) | **macOS Keychain** (no file) |
| Cookie encryption | Encrypted with OS master key | Plaintext | **Plaintext** |
| Cookie format | SQLite | SQLite | Proprietary BinaryCookies binary |
| History | SQLite | SQLite (`places.sqlite`) | SQLite (Core Data epoch) |
| Bookmark | JSON | SQLite (`places.sqlite`) | **plist** |
| Download | SQLite (`History`, shared) | SQLite (`places.sqlite`, shared) | **plist** (filtered by UUID) |
| LocalStorage | LevelDB | SQLite (`webappsstore.sqlite`) | Nested **WebKit Origins** SQLite |
| LocalStorage partitioning | No | No | **Yes** (top-frame + frame hashes) |
| CreditCard / SessionStorage | Supported | Not supported | Not supported |
| Encryption scope | Passwords, cookies, credit cards | Passwords only | Passwords only |
| Time format | WebKit microseconds since 1601 | Mixed (μs for most, ms for passwords) | Core Data seconds since 2001 |

## Related RFCs

| RFC | Topic |
|-----|-------|
| [RFC-001](001-project-architecture.md) | Project architecture and directory layout |
| [RFC-006](006-key-retrieval-mechanisms.md) | §7 covers Safari Keychain credential extraction |
| [RFC-008](008-file-acquisition-and-platform-quirks.md) | File acquisition via `filemanager.Session` |
