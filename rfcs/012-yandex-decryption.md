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
- Windows ABE v20 for Yandex — not in scope until Yandex adopts App-Bound Encryption.
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
| `crypto/crypto.go` | New generic primitive `AESGCMDecryptBlob(key, blob, aad)` — splits `[12B nonce][ct+tag]` and runs AES-GCM. Not Yandex-specific; any protocol with this wire format can use it. |
| `crypto/yandex.go` | Only exports `DecryptYandexIntermediateKey`. Holds the Yandex-specific protobuf signature strip + blob-length invariants as private constants. |
| `crypto/yandex_test.go` | Round-trip + error-path tests for `DecryptYandexIntermediateKey` and `AESGCMDecryptBlob`. |
| `browser/chromium/yandex_key.go` | `loadYandexDataKey(dbPath, masterKey)` — opens the DB, checks `active_keys.sealed_key`, reads `meta.local_encryptor_data`, returns dataKey or `errYandexMasterPasswordSet`. |
| `browser/chromium/extract_password.go` | `extractYandexPasswords` + local `yandexLoginAAD` helper. Queries `origin_url, username_element, username_value, password_element, password_value, signon_realm, date_created`, computes SHA1 AAD locally, calls `crypto.AESGCMDecryptBlob`. |
| `browser/chromium/extract_creditcard.go` | Merged file — Chromium `extractCreditCards` + Yandex `extractYandexCreditCards` + `countYandexCreditCards` + local `yandexCardAAD` helper + JSON struct types `yandexPublicData` / `yandexPrivateData`. |
| `browser/chromium/source.go` | `creditCardExtractor` wrapper (parallel to `passwordExtractor` / `extensionExtractor`); `yandexExtractors` map registers Password and CreditCard overrides. |
| `browser/chromium/chromium.go` | `countCategory` routes `CreditCard` to `countYandexCreditCards` when `cfg.Kind == ChromiumYandex` (table name differs). The extract side already dispatches through `b.extractors[cat]`. |
| `types/models.go` | `CreditCardEntry` gains `CVC` and `Comment` string fields. Chromium leaves them empty. |

### 5.1 Why Yandex logic lives inside the extractor, not the keyretriever

The keyretriever tier (V10/V11/V20) is keyed on *cipher-version prefix* — the extract side dispatches on bytes `"v10"` / `"v11"` / `"v20"`. Yandex password rows carry no such prefix; they are raw `nonce\|ct+tag`. Injecting Yandex's intermediate-key step into `keyretriever.MasterKeys` would overload the tier abstraction (which models "pick the key for this prefix"), so the intermediate key is recovered inside the Yandex extractor using the Chromium V10 key as input. The keyretriever layer is untouched.

### 5.2 Why AAD construction lives in `browser/chromium/`, not in `crypto/`

`crypto` exposes cryptographic primitives (AES, GCM, 3DES, DPAPI, PBKDF2, etc.) — things that transform bytes under a key. AAD construction for Yandex (`SHA1(origin_url ‖ \x00 ‖ …)` for passwords, raw `guid` for cards) is not cryptography; it is Yandex's per-row identification rule that happens to be bound to GCM's authentication tag. Placing it in `crypto` would leak Yandex protocol knowledge into a package that otherwise knows nothing about browsers.

The final split:

- `crypto.AESGCMDecryptBlob(key, blob, aad)` — generic AES-GCM with a caller-supplied AAD. Exported once, used by any current or future protocol that wants per-row AAD.
- `chromium.yandexLoginAAD` / `chromium.yandexCardAAD` — private helpers next to the extractor that calls them. Protocol knowledge stays with the protocol consumer.

This also keeps the `crypto` public surface small (3 extra exports: `DecryptYandexIntermediateKey`, `AESGCMDecryptBlob`, and the existing `AESGCMDecrypt`) rather than ballooning into a per-browser API.

## 6. Non-goals and deferred work

1. **Master password unseal** (#90 edge case). Profiles with a non-empty `active_keys.sealed_key` are detected and skipped with `log.Warnf`. The follow-up PR will add a `--yandex-master-password` CLI flag (or `HBD_YANDEX_MASTER_PASSWORD` env var) and the RSA-OAEP path: PBKDF2-SHA256 derives a KEK; KEK decrypts `encrypted_private_key` with AAD = `unlock_key_salt`; parsed PKCS8 RSA private key + RSA-OAEP-SHA256 decrypts `encrypted_encryption_key`; signature strip yields the dataKey.
2. **Windows ABE v20 for Yandex**. Yandex has not adopted App-Bound Encryption. If that changes, Yandex will join the RFC-010 vendor table via `crypto/windows/abe_native/com_iid.c` and the `ABERetriever` will start returning a non-empty V20 key for the `yandex` storage tag.
3. **Linux support**. Yandex Browser has no official Linux release. No `browser/browser_linux.go` entry is added.

## 7. Test strategy

All decryption math is covered by pure-Go tests that synthesize Yandex DB files using the real encryption math in reverse — no Yandex install or Windows host needed.

| File | What it validates |
|---|---|
| `crypto/yandex_test.go` | `DecryptYandexIntermediateKey` — round-trip, missing marker, truncated blob, bad signature, trailing data ignored. `AESGCMDecryptBlob` — round-trip, mismatched AAD fails GCM, blob shorter than nonce size surfaces as `errShortCiphertext`. |
| `browser/chromium/yandex_testutil_test.go` | `setupYandexPasswordDB` / `setupYandexCreditCardDB` — seal a dataKey into `meta.local_encryptor_data`, insert logins/records with matching AAD. Uses the same `yandexLoginAAD` / `yandexCardAAD` helpers as production so fixture and extractor stay in lock-step. |
| `browser/chromium/extract_password_test.go` | `TestExtractYandexPasswords` end-to-end (2 real logins round-tripped); master-password skip path; wrong master key surfaces as error. `TestYandexLoginAAD_*` covers the SHA1 shape with / without keyID. |
| `browser/chromium/extract_creditcard_test.go` | Merged file — Chromium tests for `credit_cards` plus Yandex tests: round-trip on 2-card fixture verifying Number/CVC/Comment/NickName/ExpMonth/ExpYear mapping; count on 3-row `records` table; wrong master key surfaces as error. `TestYandexCardAAD` covers guid bytes / guid+keyID. |
| `browser/chromium/chromium_test.go` | `TestExtractorsForKind` asserts `yandexExtractors` carries both `Password` and `CreditCard` entries. |

End-to-end validation on a Windows host with a real Yandex profile is expected before shipping changes that touch the decryption path; the Chromium full-sweep suite doubles as a regression gate to catch unintended impact on other Chromium forks.

## 8. Rollout

Single PR that wires all of the above; merge automatically closes #90 / #105 / #462 / #476. Follow-up PRs for master password and (if/when Yandex adopts ABE) v20 integration reference this RFC rather than reopening the decryption design question.
