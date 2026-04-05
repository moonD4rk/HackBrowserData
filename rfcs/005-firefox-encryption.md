# RFC-005: Firefox Encryption

**Author**: moonD4rk
**Status**: Living Document
**Created**: 2026-04-05

## 1. Overview

Firefox uses Mozilla's NSS (Network Security Services) for credential encryption. Unlike Chromium, which delegates key storage to the OS (DPAPI, Keychain, D-Bus), Firefox manages its own encryption entirely within the profile directory via `key4.db`. This makes Firefox encryption **platform-agnostic** — the same derivation logic works on Windows, macOS, and Linux.

Only passwords are encrypted. Cookies, history, bookmarks, downloads, extensions, and localStorage are all stored in plaintext. See [RFC-004](004-firefox-data-storage.md) for storage details.

## 2. Master Key Derivation (key4.db)

### 2.1 Database Structure

`key4.db` is a SQLite database containing two relevant tables:

| Table | Purpose | Key Columns |
|-------|---------|-------------|
| `metaData` | Stores the global salt and an encrypted integrity marker | `item1` (global salt), `item2` (encrypted "password-check" string) |
| `nssPrivate` | Stores encrypted master key candidates | `a11` (PBE-encrypted key blob), `a102` (key type tag) |

The `nssPrivate` table may contain multiple rows (certificates, other NSS objects). Only rows where `a102` matches a specific 16-byte type tag (`{0xF8, 0x00, ...0x01}`) are actual master key entries.

### 2.2 Derivation Flow

1. **Read metaData** — extract the global salt and encrypted password-check marker from the row where `id = 'password'`.
2. **Verify integrity** — decrypt the password-check marker using the global salt via ASN1 PBE. The plaintext must contain the string `"password-check"`. This confirms the database is valid and the empty-password assumption holds (Firefox uses an empty master password by default).
3. **Decrypt key candidates** — for each `nssPrivate` row matching the type tag, decrypt the `a11` blob using the global salt via ASN1 PBE. The result must be at least 24 bytes.
4. **Validate against logins** — if `logins.json` is available, each candidate key is tested by attempting to decrypt an actual login entry (both username and password). The first key that succeeds is selected. This prevents selecting the wrong candidate when multiple keys exist.

## 3. ASN1 PBE Types

Firefox wraps all encrypted data in ASN1 structures. Three PBE (Password-Based Encryption) types are used, each with a distinct ASN1 layout:

| PBE Type | Used For | Cipher | Key Derivation |
|----------|----------|--------|----------------|
| `privateKeyPBE` | Master key entries in `nssPrivate` | 3DES-CBC | SHA1 + HMAC-SHA1 custom NSS derivation |
| `passwordCheckPBE` | Integrity marker in `metaData` | AES-256-CBC | PBKDF2-SHA256 |
| `credentialPBE` | Encrypted fields in `logins.json` | 3DES-CBC or AES-256-CBC | Master key used directly (no derivation) |

The `key` parameter has different semantics depending on the PBE type:

- **privateKeyPBE / passwordCheckPBE**: the key parameter is the **global salt**, used as input to key derivation.
- **credentialPBE**: the key parameter is the **already-derived master key**, used directly for decryption.

`NewASN1PBE()` auto-detects the type by attempting to unmarshal the raw bytes against each ASN1 structure in order.

### 3.1 privateKeyPBE Key Derivation

The NSS PBE-SHA1-3DES derivation produces a 40-byte derived key from the global salt and an entry-specific salt:

```
hp    = SHA1(globalSalt)
ck    = SHA1(hp || entrySalt)
k1    = HMAC-SHA1(ck, pad(entrySalt,20) || entrySalt)
k2    = HMAC-SHA1(ck, HMAC-SHA1(ck, pad(entrySalt,20)) || entrySalt)
dk    = k1 || k2                     // 40 bytes
key   = dk[:24], iv = dk[32:40]      // 3DES key + IV
```

### 3.2 passwordCheckPBE Key Derivation

Uses standard PBKDF2 with SHA-256 and parameters embedded in the ASN1 structure (entry salt, iteration count, key size). The IV is reconstructed by prepending the ASN.1 OCTET STRING header (`0x04 0x0E`) to the 14-byte IV value from the parsed structure, yielding a 16-byte AES IV.

## 4. Password Decryption

### 4.1 3DES-CBC (Firefox < 144)

Legacy Firefox versions encrypt login credentials with 3DES-CBC. The `credentialPBE` ASN1 structure wraps the ciphertext with its own IV:

```
| ASN1 OID + params | IV    | 3DES-CBC ciphertext (PKCS5 padded) |
|--------------------|-------|------------------------------------|
| variable           | 8B    | remaining bytes                    |
```

Decryption details:
- **Key**: the first 24 bytes of the master key (derived from `key4.db`, see Section 2)
- **IV**: 8-byte IV embedded in the ASN1 structure
- **Algorithm**: Triple DES in CBC mode with PKCS5 padding
- **Padding removal**: after decryption, PKCS5 padding bytes are stripped. The last byte of plaintext indicates how many padding bytes to remove (1-8).

3DES uses three independent 8-byte DES keys (k1, k2, k3) packed into the 24-byte key:

```
| k1 (DES key 1) | k2 (DES key 2) | k3 (DES key 3) |
|-----------------|-----------------|-----------------|
| 8B              | 8B              | 8B              |
```

Encryption: `E(k1) → D(k2) → E(k3)`. Decryption: `D(k3) → E(k2) → D(k1)`.

### 4.2 AES-256-CBC (Firefox 144+)

Starting from [Firefox 144](https://www.firefox.com/en-US/firefox/144.0/releasenotes/) (January 2025), Mozilla migrated password encryption from 3DES to AES-256-CBC for stronger security. The ASN1 structure has the same layout but with a larger IV:

```
| ASN1 OID + params | IV    | AES-256-CBC ciphertext (PKCS5 padded) |
|--------------------|-------|---------------------------------------|
| variable           | 16B   | remaining bytes                       |
```

Decryption details:
- **Key**: the full master key (32 bytes for AES-256)
- **IV**: 16-byte IV embedded in the ASN1 structure
- **Algorithm**: AES-256 in CBC mode with PKCS5 padding
- **Cipher selection**: the cipher is inferred from the **IV length** rather than checking OIDs — 8-byte IV means 3DES, 16-byte IV means AES-256-CBC. This allows the same code path to handle both old and new Firefox profiles.

### 4.3 Pipeline

Each encrypted login field (`encryptedUsername`, `encryptedPassword` in `logins.json`) follows the same decryption pipeline:

```
logins.json
  → encryptedUsername / encryptedPassword (base64 string)

| base64 encoded string                                    |
|----------------------------------------------------------|
                        ↓ base64 decode
| raw ASN1 DER bytes                                       |
|----------------------------------------------------------|
                        ↓ ASN1 parse (auto-detect credentialPBE)
| IV (8B or 16B) | ciphertext                              |
|----------------------------------------------------------|
                        ↓ decrypt (3DES or AES-256 based on IV length)
| plaintext + PKCS5 padding                                |
|----------------------------------------------------------|
                        ↓ strip PKCS5 padding
| plaintext (UTF-8 string)                                 |
|----------------------------------------------------------|
```

The master key is passed through unchanged — `credentialPBE` uses the key directly without further derivation (unlike `privateKeyPBE` and `passwordCheckPBE` which derive from the global salt).

## Related RFCs

| RFC | Topic |
|-----|-------|
| [RFC-004](004-firefox-data-storage.md) | Firefox data file locations and storage formats |
| [RFC-006](006-key-retrieval-mechanisms.md) | Platform-specific master key retrieval (Chromium only — Firefox is self-contained) |
