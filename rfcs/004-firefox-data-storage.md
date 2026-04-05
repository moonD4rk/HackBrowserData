# RFC-004: Firefox Data Storage

**Author**: moonD4rk
**Status**: Living Document
**Created**: 2026-04-05

## 1. Profile Structure

Firefox stores per-user data in **profile directories** beneath a platform-specific root (e.g. `~/Library/Application Support/Firefox/Profiles/` on macOS). Each profile directory has a random-prefix name like `97nszz88.default-release`.

Profile discovery enumerates subdirectories of the root and accepts any directory that contains at least one known data file. Unlike Chromium (which looks for a `Preferences` sentinel), Firefox validation simply checks for the presence of any source file from the table below.

## 2. Data File Locations

All paths are relative to the profile directory.

| Category | File | Format |
|----------|------|--------|
| Password | `logins.json` | JSON |
| Cookie | `cookies.sqlite` | SQLite |
| History | `places.sqlite` | SQLite |
| Download | `places.sqlite` | SQLite |
| Bookmark | `places.sqlite` | SQLite |
| Extension | `extensions.json` | JSON |
| LocalStorage | `webappsstore.sqlite` | SQLite |

History, Download, and Bookmark all share `places.sqlite` but query different tables within it. Firefox does not support CreditCard or SessionStorage extraction.

The master encryption key is stored separately in `key4.db` (see [RFC-005](005-firefox-encryption.md)).

## 3. Data Storage Formats

### 3.1 Passwords (logins.json)

Passwords are stored as a JSON file with a top-level `logins` array. Each entry contains:

- `formSubmitURL` / `hostname` — the login URL (formSubmitURL preferred, hostname as fallback)
- `encryptedUsername` — base64-encoded, ASN1 PBE-encrypted username
- `encryptedPassword` — base64-encoded, ASN1 PBE-encrypted password
- `timeCreated` — creation timestamp in **milliseconds**

Decryption pipeline: base64 decode the field, parse as ASN1 PBE structure, decrypt with the master key.

### 3.2 Cookies (cookies.sqlite)

Cookies are **not encrypted** — values are stored in plaintext.

```sql
SELECT name, value, host, path, creationTime, expiry, isSecure, isHttpOnly
FROM moz_cookies
```

The database must be opened with `journal_mode=off` to avoid locking conflicts with a running Firefox instance.

### 3.3 History (places.sqlite)

```sql
SELECT url, COALESCE(last_visit_date, 0), COALESCE(title, ''), visit_count
FROM moz_places
```

The `last_visit_date` column uses **microseconds** since epoch.

### 3.4 Downloads (places.sqlite)

Downloads use the `moz_annos` annotation table joined with `moz_places`:

```sql
SELECT place_id, GROUP_CONCAT(content), url, dateAdded
FROM (SELECT * FROM moz_annos INNER JOIN moz_places ON moz_annos.place_id = moz_places.id)
t GROUP BY place_id
```

Download metadata is stored as a concatenated string: `target_path,{json}` where the JSON portion contains `fileSize` and `endTime`.

### 3.5 Bookmarks (places.sqlite)

```sql
SELECT id, url, type, dateAdded, COALESCE(title, '')
FROM (SELECT * FROM moz_bookmarks INNER JOIN moz_places ON moz_bookmarks.fk = moz_places.id)
```

The `type` field distinguishes URL bookmarks (1) from folders.

### 3.6 Extensions (extensions.json)

Extensions are read from the `addons` array. Only entries with `location == "app-profile"` are included (user-installed extensions). Fields extracted: `defaultLocale.name`, `id`, `version`, `defaultLocale.description`, `defaultLocale.homepageURL`, `active`.

### 3.7 LocalStorage (webappsstore.sqlite)

```sql
SELECT originKey, key, value FROM webappsstore2
```

The `originKey` column uses a **reversed-host format**: `moc.buhtig.:https:443` represents `https://github.com:443`. The host portion is byte-reversed and dot-suffixed; the remaining fields are scheme and port.

## 4. Time Formats

Firefox uses inconsistent timestamp units across data types. All are Unix epoch-based.

| Data Type | Unit | Conversion |
|-----------|------|------------|
| Cookies (`creationTime`) | Microseconds | / 1,000,000 |
| Cookies (`expiry`) | Seconds | direct |
| History (`last_visit_date`) | Microseconds | / 1,000,000 |
| Downloads (`dateAdded`) | Microseconds | / 1,000,000 |
| Bookmarks (`dateAdded`) | Microseconds | / 1,000,000 |
| Passwords (`timeCreated`) | Milliseconds | / 1,000 |

## 5. Key Differences from Chromium

| Aspect | Chromium | Firefox |
|--------|----------|---------|
| Profile naming | Named directories (`Default`, `Profile 1`) | Random-prefix (`97nszz88.default-release`) |
| Profile detection | `Preferences` sentinel file | Any known source file present |
| Password storage | SQLite (`Login Data`) | JSON (`logins.json`) |
| Cookie encryption | Encrypted with master key | **Plaintext** |
| Shared database | Separate files per category | `places.sqlite` shared by History/Download/Bookmark |
| LocalStorage | LevelDB | SQLite (`webappsstore.sqlite`) |
| CreditCard support | Yes | No |
| SessionStorage support | Yes | No |
| Encryption scope | Passwords, cookies, credit cards | **Passwords only** (see [RFC-005](005-firefox-encryption.md)) |

## Related RFCs

| RFC | Topic |
|-----|-------|
| [RFC-005](005-firefox-encryption.md) | Firefox NSS encryption and master key derivation |
| [RFC-008](008-file-acquisition-and-platform-quirks.md) | File acquisition and platform quirks |
