# RFC-002: Chromium Data Storage

**Author**: moonD4rk
**Status**: Living Document
**Created**: 2026-04-05

## 1. Data File Locations

All paths are relative to the profile directory (e.g. `~/.config/google-chrome/Default/`).

| Category | Candidate Paths (priority order) | Format |
|----------|----------------------------------|--------|
| Password | `Login Data` | SQLite |
| Cookie | `Network/Cookies`, `Cookies` | SQLite |
| History | `History` | SQLite |
| Download | `History` (same file) | SQLite |
| Bookmark | `Bookmarks` | JSON |
| CreditCard | `Web Data` | SQLite |
| Extension | `Secure Preferences` | JSON |
| LocalStorage | `Local Storage/leveldb/` | LevelDB dir |
| SessionStorage | `Session Storage/` | LevelDB dir |

Cookies have two candidate paths because older Chromium versions stored cookies at `<profile>/Cookies`, while newer versions moved them to `<profile>/Network/Cookies`. The first existing path wins.

## 2. Browser Variants

### 2.1 Yandex

Yandex overrides two file names from the standard Chromium layout:

| Category | Standard Chromium | Yandex |
|----------|-------------------|--------|
| Password | `Login Data` | `Ya Passman Data` |
| CreditCard | `Web Data` | `Ya Credit Cards` |

Yandex also uses `action_url` instead of `origin_url` in its password SQL query.

**Important limitation**: Yandex passwords and cookies currently cannot be decrypted because Yandex uses its own proprietary encryption algorithm. Only non-encrypted categories (bookmarks, history, downloads, extensions, storage) produce useful results.

### 2.2 Opera

Opera differs from standard Chromium in two ways:

- **Extension key**: Opera stores extension settings under `extensions.opsettings` in Secure Preferences, instead of the standard `extensions.settings`.
- **Windows path**: Opera uses `AppData/Roaming` rather than `AppData/Local`, unlike most Chromium browsers.
- **Flat layout**: Older Opera versions store data files directly in the user data directory without profile subdirectories (see Section 3).

## 3. Profile Discovery

Chromium supports multiple profiles (Default, Profile 1, Profile 2, ...) under a single user data directory. Profile discovery identifies which subdirectories are actual profiles versus internal directories like `Crashpad` or `ShaderCache`.

A directory is recognized as a profile if it contains a `Preferences` file. This convention follows Chromium's own source code -- Chromium creates a per-profile `Preferences` file on first use, making it a reliable marker even in early Chromium versions. Tencent-based browsers (QQ Browser, Sogou Explorer) use `Preferences_02` instead, which is also checked.

Certain directories are always skipped: `System Profile`, `Guest Profile`, and `Snapshot`.

**Flat layout fallback**: If no profile subdirectories are found, the user data directory itself is checked for any known source file. This handles Opera-style browsers that store data alongside `Local State` in the base directory.

## 4. Data Storage Formats

### 4.1 Passwords (Login Data -- SQLite)

```sql
SELECT origin_url, username_value, password_value, date_created FROM logins
```

The `password_value` column contains encrypted bytes. See [RFC-003](003-chromium-encryption.md) for decryption.

### 4.2 Cookies (Cookies -- SQLite)

```sql
SELECT name, encrypted_value, host_key, path,
    creation_utc, expires_utc, is_secure, is_httponly,
    has_expires, is_persistent FROM cookies
```

The `encrypted_value` column contains encrypted bytes. Chrome 130+ (cookie DB schema version 24) prepends `SHA256(host_key)` to the cookie value before encryption as a cross-domain replay mitigation. After decryption, the cookie value layout is:

```
| SHA256(host_key) | actual cookie value |
|------------------|---------------------|
| 32B              | remaining bytes     |
```

The first 32 bytes are verified against `SHA256(host_key)` and stripped if they match. If the decrypted value is shorter than 32 bytes or the hash does not match, the value is returned as-is (pre-Chrome 130 behavior).

### 4.3 Bookmarks (Bookmarks -- JSON)

A JSON file with a `roots` object containing bookmark trees (bookmark_bar, other, synced). Each node has a `type` ("url" or "folder"), `name`, `url`, and `date_added`. Folder nodes contain a `children` array, forming a recursive tree that is walked to collect all URL entries.

### 4.4 History (History -- SQLite)

```sql
SELECT url, title, visit_count, last_visit_time FROM urls
```

No encrypted fields. Results are sorted by visit count (descending).

### 4.5 Downloads (History -- SQLite, same file)

```sql
SELECT target_path, tab_url, total_bytes, start_time, end_time, mime_type FROM downloads
```

No encrypted fields. Shares the same `History` SQLite database as browsing history.

### 4.6 Credit Cards (Web Data -- SQLite)

```sql
SELECT guid, name_on_card, expiration_month, expiration_year,
    card_number_encrypted, nickname, billing_address_id FROM credit_cards
```

The `card_number_encrypted` column contains encrypted bytes.

### 4.7 Extensions (Secure Preferences -- JSON)

The `Secure Preferences` file contains extension metadata under `extensions.settings` (or variant-specific keys). Each extension entry includes a `manifest` object with name, description, version, and homepage URL. System/component extensions (location 5 or 10) are filtered out.

Extension enabled state is determined by `disable_reasons` (modern Chrome: empty array = enabled) or `state` (older Chrome: 1 = enabled).

### 4.8 LocalStorage / SessionStorage (LevelDB)

Both use LevelDB directories, but with different key encoding schemes.

**LocalStorage** keys use a binary format: a `_` prefix byte, followed by the origin URL, a null separator, and a Chromium-encoded string key. The string encoding uses a format byte: `0x01` for Latin-1, `0x00` for UTF-16 LE. Values follow the same encoding. Metadata entries (`META:`, `METAACCESS:`) and `VERSION` keys are recognized but not treated as user data.

**SessionStorage** uses a two-pass approach. First, `namespace-<guid>-<origin>` entries map GUIDs to origins. Then, `map-<map_id>-<key_name>` entries contain the actual data with raw UTF-16 LE values (no format byte prefix).

## 5. Time Format

Chromium uses WebKit epoch timestamps: microseconds since 1601-01-01 00:00:00 UTC. This applies to `date_created`, `creation_utc`, `expires_utc`, `last_visit_time`, `start_time`, `end_time`, and `date_added`. To convert to Unix time, subtract 11644473600000000 microseconds (the offset between 1601 and 1970).

## Related RFCs

| RFC | Topic |
|-----|-------|
| [RFC-003](003-chromium-encryption.md) | Chromium encryption mechanisms per platform |
| [RFC-006](006-key-retrieval-mechanisms.md) | Platform-specific master key retrieval |
| [RFC-008](008-file-acquisition-and-platform-quirks.md) | File acquisition and platform quirks |
