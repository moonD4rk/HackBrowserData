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

1. **Master key**: read `Local State`, base64-decode `os_crypt.encrypted_key`, strip the `DPAPI` prefix, and unwrap it via DPAPI (Windows) or Keychain (macOS). Yields 32 bytes.
2. **Open DB**: open the target SQLite file (a temp copy is used to avoid lock contention if the browser is running).
3. **Master-password gate**: `SELECT sealed_key FROM active_keys`. Non-empty → log a warning and skip the profile (v1 limitation — RSA-OAEP unseal deferred). Table missing (credit-card DB) or empty value → continue.
4. **Data key**: `SELECT value FROM meta WHERE key='local_encryptor_data'`. Find the `"v10"` byte sequence, take the 96 bytes that follow, split into 12B nonce + 84B (ciphertext+tag), AES-GCM-decrypt with the master key (no AAD), strip the 4-byte protobuf signature `08 01 12 20`, keep the first 32 bytes.
5. **Per-row decryption**: for each row, compute AAD (see §4.4), split `[12B nonce][ct+tag]`, AES-GCM-decrypt with the data key under that AAD.

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

## 5. Layering rationale

### 5.1 Yandex-specific derivation stays in the extract path, not the key-retrieval layer

The key-retrieval layer dispatches on cipher-version prefix — `v10` / `v11` / `v20`. Yandex password rows carry no such prefix; they are raw `[nonce][ct+tag]`. Folding Yandex's intermediate-key step into the prefix dispatcher would overload an abstraction that is purely "pick the key for this byte prefix". The intermediate-key unwrap therefore lives alongside the Yandex extractor and consumes the standard Chromium master key as input; the prefix dispatcher is untouched.

### 5.2 AAD construction belongs with the consumer, not the crypto layer

The crypto layer exposes cryptographic primitives — transforms of bytes under a key (AES, GCM, 3DES, DPAPI, PBKDF2). Yandex's AAD rules (SHA-1 over five form fields for passwords, the row's GUID for cards) are not cryptography; they are Yandex's per-row identification scheme that happens to be bound to GCM's authentication tag. Placing them in the crypto layer would leak product-specific knowledge into a layer that otherwise sees only bytes and keys.

The final split:

- A single generic AES-GCM-with-AAD primitive in the crypto layer. Any current or future protocol that needs per-row AAD can reuse it without the crypto layer growing per-product surface.
- Yandex-specific AAD helpers next to the consumer that builds the AAD inputs. Product knowledge stays with the product.

This keeps the crypto surface minimal — the only Yandex symbol it owns is the intermediate-key unwrap, because that one function genuinely *is* cryptography (it strips a protobuf frame and decrypts AES-GCM).

## 6. Non-goals and deferred work

1. **Master-password unseal** (#90 edge case). Profiles with a non-empty `active_keys.sealed_key` are detected and skipped with a warning. A follow-up RFC will cover the RSA-OAEP path: PBKDF2-SHA256 derives a KEK; the KEK decrypts `encrypted_private_key` with AAD = `unlock_key_salt`; the resulting PKCS8 RSA private key + RSA-OAEP-SHA256 decrypts `encrypted_encryption_key`; the signature strip then yields the dataKey.
2. **Windows ABE v20 for Yandex**. Yandex has not adopted App-Bound Encryption. If that changes, Yandex joins the RFC-010 vendor table and the ABE path begins returning a non-empty v20 key for Yandex ciphertexts.
3. **Linux support**. Yandex Browser has no official Linux release, so there is no Linux code path to add.

## 7. Test strategy

Decryption math is covered by cross-platform unit tests that build synthetic DBs by running the encryption path in reverse — no real Yandex install or Windows host is required. Coverage spans:

- Intermediate-key unwrap: round-trip, missing `v10` marker, truncated blob, bad protobuf signature, trailing bytes ignored.
- AES-GCM-with-AAD primitive: round-trip, mismatched AAD surfaces as authentication failure, under-sized blob surfaces as a distinct error.
- Password extraction: round-trip on multi-row fixtures, master-password skip path, wrong master key surfaces as error.
- Credit-card extraction: round-trip on multi-card fixtures verifying every JSON field maps to the output schema; count; wrong master key surfaces as error.
- AAD formulas: SHA-1 field concatenation (passwords), GUID bytes (cards), both with and without a master-password keyID appended.

End-to-end validation on a Windows host with a real Yandex profile is expected before shipping changes that touch the decryption path; the existing Chromium full-sweep doubles as a regression gate against unintended impact on other Chromium forks.

## 8. Rollout

Single PR that wires all of the above; merge automatically closes #90 / #105 / #462 / #476. Follow-up PRs for master password and (if/when Yandex adopts ABE) v20 integration reference this RFC rather than reopening the decryption design question.
