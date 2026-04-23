# RFC-012: Yandex Browser Decryption

**Author**: moonD4rk
**Status**: Living Document
**Created**: 2026-04-22
**Last updated**: 2026-04-22

## 1. Overview

Yandex Browser is a Chromium fork, but its saved-credential encryption diverges from the Chromium reference in three ways that together make a plain Chromium extractor produce zero plaintext:

1. The Chromium master key (DPAPI on Windows, Keychain on macOS) does not decrypt `password_value` directly — it decrypts a per-DB *intermediate* key stored in `meta.local_encryptor_data`. That intermediate key is what actually decrypts rows.
2. Each row's AES-GCM ciphertext is sealed with row-specific Additional Authenticated Data (AAD). A password row's AAD is a SHA-1 digest over five form fields joined by `\x00`; a credit-card row's AAD is the row's `guid`. AAD mismatch → GCM tag failure → empty plaintext.
3. Credit cards live in `records(guid, public_data, private_data)` — two JSON blobs — not Chromium's flat `credit_cards` table.

This RFC documents the on-disk layout, the decryption math, and how the integration plugs into the existing Chromium extract pipeline without perturbing the v10/v11/v20 paths that the rest of HackBrowserData depends on.

Resolved issues: #90 (feature request), #105 / #462 / #476 (downstream bug reports against the incomplete skeleton that was merged before this RFC).

Related RFCs:

- [RFC-003](003-chromium-encryption.md) — Chromium cipher versions (v10 / v11 / v20)
- [RFC-006](006-key-retrieval-mechanisms.md) — master-key retrieval chain

Deferred to a follow-up RFC / PR:

- Master-password (RSA-OAEP + PBKDF2) unseal path.
- Windows ABE v20 for Yandex; as of 2026-04 Yandex has not adopted App-Bound Encryption.
- Linux support; Yandex Browser has no official Linux build.

## 2. Protocol differences at a glance

| Layer | Standard Chromium | Yandex |
|---|---|---|
| Master key | `os_crypt.encrypted_key` in `Local State`, unwrapped via DPAPI / Keychain | Same |
| Decryption key used per row | Master key directly | Intermediate 32-byte key stored per-DB in `meta.local_encryptor_data` |
| Key wrapper format | `"v10"\|nonce\|ct+tag` (or DPAPI blob) | `"v10"\|nonce\|ct+tag`, plaintext prefixed by 4B protobuf signature `08 01 12 20`, 32B key follows |
| Password DB file | `Login Data` (table: `logins`) | `Ya Passman Data` (table: `logins`) |
| Password ciphertext | `"v10"\|nonce\|ct+tag`, AAD = empty | No prefix; raw `nonce\|ct+tag`; AAD = SHA1(origin_url ‖ \x00 ‖ username_element ‖ \x00 ‖ username_value ‖ \x00 ‖ password_element ‖ \x00 ‖ signon_realm) |
| Credit-card DB file | `Web Data` (table: `credit_cards`) | `Ya Credit Cards` (table: `records`) |
| Credit-card layout | Columns: `name_on_card`, `expiration_month`, `card_number_encrypted`, … | JSON: `public_data` (plaintext) + `private_data` (AES-GCM sealed JSON, AAD = `guid`) |
| Master password | n/a | Optional; when set, `active_keys.sealed_key` holds an RSA-OAEP envelope (deferred) |

## 3. On-disk layout

### 3.1 `meta.local_encryptor_data`

```
[protobuf preamble bytes...] "v10" [12B nonce] [68B plaintext + 16B GCM tag]
```

The 68-byte plaintext (decrypted with the Chromium master key, empty AAD) has the shape:

```
08 01 12 20  | KK KK ... KK  (32 bytes)  | padding / extra protobuf fields
^ signature  | ^ data-encryption key
```

The data-encryption key is the first 32 bytes after the signature; trailing bytes are ignored. The fixed 96-byte region after `"v10"` is a Yandex invariant (the reference implementation slices `[:96]` unconditionally) and is checked as a minimum length.

### 3.2 Password row (`logins.password_value`)

```
[12B nonce] [ciphertext] [16B GCM tag]
```

No version prefix. AAD binds five form columns:

```
SHA1(origin_url ‖ 0x00 ‖ username_element ‖ 0x00 ‖ username_value ‖ 0x00 ‖ password_element ‖ 0x00 ‖ signon_realm)
```

When a master password is set, the sealed keyID is appended after the SHA-1 sum. v1 always passes `nil` and skips sealed profiles.

### 3.3 Credit card row (`records.private_data`)

Same byte shape as passwords but AAD = the row's `guid` bytes (plus optional keyID). Decrypted plaintext is a JSON object with `full_card_number`, `pin_code`, `secret_comment`. The sibling `public_data` column is plaintext JSON with `card_holder`, `card_title`, `expire_date_month`, `expire_date_year`.

## 4. Architecture

### 4.1 Two-level key hierarchy

Yandex adds a second key layer on top of the standard Chromium key. The Chromium master key — unwrapped from `Local State` via DPAPI (Windows) or Keychain (macOS) — never decrypts row ciphertext directly. Instead, each target SQLite database carries its own *data key* in `meta.local_encryptor_data`, and only that data key decrypts row-level ciphertext. The master key's only job is to unwrap the data key.

### 4.2 Recovery steps

For every target DB (`Ya Passman Data` for passwords, `Ya Credit Cards` for cards), the extractor runs the same five steps:

1. **Master key**: read `Local State`, base64-decode `os_crypt.encrypted_key`, strip the `DPAPI` prefix, call `CryptUnprotectData` (Windows) or read from Keychain (macOS). Yields 32 bytes.
2. **Open DB**: `session.Acquire` has already copied the target SQLite file to a temp path; `loadYandexDataKey` opens it there.
3. **Master-password gate**: `SELECT sealed_key FROM active_keys`. Non-empty → return `errYandexMasterPasswordSet`; the caller logs a warning and skips the profile (v1 limitation). Table missing (credit-card DB) or empty value → continue.
4. **Data key**: `SELECT value FROM meta WHERE key='local_encryptor_data'`. Find the `"v10"` byte sequence, take the 96 bytes that follow, split into 12B nonce + 84B (ciphertext+tag). AES-GCM-decrypt with the master key (no AAD). Strip the 4-byte protobuf signature `08 01 12 20`. Keep the first 32 bytes.
5. **Per-row decryption**: for each row, compute AAD (see §4.4), split `[12B nonce][ct+tag]`, call `AESGCMDecryptWithAAD(dataKey, nonce, ct, aad)`.

### 4.3 Key hierarchy

| Level | Key | Origin | Scope |
|---|---|---|---|
| 1 | Chromium master key | `Local State` → DPAPI / Keychain | Whole profile (shared with cookies, history, etc.) |
| 2a | Passwords data key | `Ya Passman Data` → `meta.local_encryptor_data` | `logins` rows in this DB only |
| 2b | Credit cards data key | `Ya Credit Cards` → `meta.local_encryptor_data` | `records` rows in this DB only |

### 4.4 Per-category decryption inputs

| Category | DB file | Table / column | Ciphertext layout | AAD |
|---|---|---|---|---|
| Password | `Ya Passman Data` | `logins.password_value` | `[12B nonce][ct+tag]` | `SHA1(origin_url ‖ \x00 ‖ username_element ‖ \x00 ‖ username_value ‖ \x00 ‖ password_element ‖ \x00 ‖ signon_realm)` |
| Credit card | `Ya Credit Cards` | `records.private_data` | `[12B nonce][ct+tag]` | raw `guid` bytes |

Credit-card plaintext is a JSON object (`full_card_number`, `pin_code`, `secret_comment`) that the extractor unmarshals into `CreditCardEntry`. The sibling `records.public_data` is plaintext JSON (`card_holder`, `card_title`, `expire_date_year`, `expire_date_month`) and needs no decryption.

### 4.5 Independence property

The two level-2 data keys are unwrapped from **different** `meta.local_encryptor_data` blobs — one per DB. This matters in two ways:

- A profile with a master password blocks passwords (step 3 trips) but credit cards can still decrypt, because the card DB has no `active_keys` table.
- Corruption of one DB's meta blob does not cascade to the other.

Both data keys still ultimately derive from the same level-1 Chromium master key, so loss of DPAPI (e.g., Windows user-profile rebuild) breaks both simultaneously.

## 5. Code layout

| Path | Role |
|---|---|
| `crypto/yandex.go` | Pure-Go primitives: `DecryptYandexIntermediateKey`, `AESGCMDecryptWithAAD`, `YandexLoginAAD`, `YandexCardAAD`, `YandexSignature`. Cross-platform, unit-testable on any host. |
| `crypto/yandex_test.go` | Round-trip tests using synthesized blobs (no Yandex install required). |
| `browser/chromium/yandex_key.go` | `loadYandexDataKey(dbPath, masterKey)` — opens the DB, checks `active_keys`, reads `meta.local_encryptor_data`, returns dataKey or `errYandexMasterPasswordSet`. |
| `browser/chromium/extract_password.go` | `extractYandexPasswords` — queries `origin_url, username_element, username_value, password_element, password_value, signon_realm, date_created`; computes per-row AAD; decrypts. |
| `browser/chromium/extract_creditcard_yandex.go` | `extractYandexCreditCards` + `countYandexCreditCards` — queries `records`; decrypts `private_data` with guid-AAD; parses both JSON blobs. |
| `browser/chromium/source.go` | `creditCardExtractor` wrapper (parallel to `passwordExtractor` / `extensionExtractor`); `yandexExtractors` map registers Password and CreditCard overrides. |
| `browser/chromium/chromium.go` | `countCategory` routes `CreditCard` to `countYandexCreditCards` when `cfg.Kind == ChromiumYandex` (table name differs). The extract side already dispatches through `b.extractors[cat]`. |
| `types/models.go` | `CreditCardEntry` gains `CVC` and `Comment` string fields. Chromium leaves them empty. |

### 5.1 Why Yandex logic lives inside the extractor, not the keyretriever

The keyretriever tier (V10/V11/V20) is keyed on *cipher-version prefix* — the extract side dispatches on bytes `"v10"` / `"v11"` / `"v20"`. Yandex password rows carry no such prefix; they are raw `nonce\|ct+tag`. Injecting Yandex's intermediate-key step into `keyretriever.MasterKeys` would overload the tier abstraction (which models "pick the key for this prefix"), so the intermediate key is recovered inside the Yandex extractor using the Chromium V10 key as input. The keyretriever layer is untouched.

### 5.2 Why `AESGCMDecryptWithAAD` is a new function rather than an extension of `AESGCMDecrypt`

`crypto.AESGCMDecrypt` is called from the v10 Chromium path with an implicit `aad = nil` semantics and is covered by the Chromium regression suite. Changing its signature or threading an AAD parameter through would ripple through every extractor. A dedicated `AESGCMDecryptWithAAD` keeps the Chromium call sites byte-identical and confines the new behavior to Yandex.

## 6. Non-goals and deferred work

1. **Master password unseal** (#90 edge case). Profiles with a non-empty `active_keys.sealed_key` are detected and skipped with `log.Warnf`. The follow-up PR will add a `--yandex-master-password` CLI flag (or `HBD_YANDEX_MASTER_PASSWORD` env var) and the RSA-OAEP path: PBKDF2-SHA256 derives a KEK; KEK decrypts `encrypted_private_key` with AAD = `unlock_key_salt`; parsed PKCS8 RSA private key + RSA-OAEP-SHA256 decrypts `encrypted_encryption_key`; signature strip yields the dataKey.
2. **Windows ABE v20 for Yandex**. Yandex has not adopted App-Bound Encryption. If that changes, Yandex will join the RFC-010 vendor table via `crypto/windows/abe_native/com_iid.c` and the `ABERetriever` will start returning a non-empty V20 key for the `yandex` storage tag.
3. **Linux support**. Yandex Browser has no official Linux release. No `browser/browser_linux.go` entry is added.

## 7. Test strategy

All decryption math is covered by pure-Go tests that synthesize Yandex DB files using the real encryption math in reverse — no Yandex install or Windows host needed.

| File | What it validates |
|---|---|
| `crypto/yandex_test.go` | `DecryptYandexIntermediateKey` round-trip, missing marker, truncated blob, bad signature, trailing data ignored; `AESGCMDecryptWithAAD` round-trip + bad AAD + bad nonce length; `YandexLoginAAD` / `YandexCardAAD` output shape with/without keyID. |
| `browser/chromium/yandex_testutil_test.go` | `setupYandexPasswordDB` / `setupYandexCreditCardDB` — seal a dataKey into `meta.local_encryptor_data`, insert logins/records with matching AAD. |
| `browser/chromium/extract_password_test.go` | `TestExtractYandexPasswords` end-to-end; master-password skip path; wrong master key surfaces as error. |
| `browser/chromium/extract_creditcard_yandex_test.go` | Round-trip on 2-card fixture verifying Number/CVC/Comment/NickName/ExpMonth/ExpYear mapping; count on 3-row table; wrong master key surfaces as error. |
| `browser/chromium/chromium_test.go` | `TestExtractorsForKind` asserts `yandexExtractors` carries both `Password` and `CreditCard` entries. |

Windows-host validation (out-of-tree, per `CLAUDE.local.md`): `make build-windows` → deploy to sandbox → `hbd.exe -v -b yandex` → verify non-empty `password.json` / `creditcard.json` and no regression in the 574-cookie 13-browser full-sweep baseline.

## 8. Rollout

Single PR that wires all of the above; merge automatically closes #90 / #105 / #462 / #476. Follow-up PRs for master password and (if/when Yandex adopts ABE) v20 integration reference this RFC rather than reopening the decryption design question.
