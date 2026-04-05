# RFC-003: Chromium Encryption

**Author**: moonD4rk
**Status**: Living Document
**Created**: 2026-04-05

## 1. Overview

Chromium encrypts sensitive fields in three data categories: passwords (`password_value`), cookies (`encrypted_value`), and credit cards (`card_number_encrypted`). The encryption algorithm varies by platform -- macOS and Linux use AES-128-CBC with a PBKDF2-derived key, while Windows uses AES-256-GCM with a DPAPI-protected key.

Non-sensitive categories (history, bookmarks, downloads, extensions, storage) are stored in plaintext and do not require decryption.

## 2. Cipher Version Detection

Every encrypted value begins with a 3-byte prefix that identifies the cipher version:

| Prefix | Version | Meaning |
|--------|---------|---------|
| `v10` | CipherV10 | Chrome 80+ standard encryption (AES-GCM on Windows, AES-CBC on macOS/Linux) |
| `v20` | CipherV20 | Chrome 127+ App-Bound Encryption |
| (none) | CipherDPAPI | Pre-Chrome 80 raw DPAPI encryption (Windows only, no prefix) |

If the ciphertext is shorter than 3 bytes or the prefix is unrecognized, it is treated as legacy DPAPI.

## 3. macOS Encryption

Chromium on macOS stores a per-browser secret in the macOS Keychain (e.g. "Chrome Safe Storage", "Brave Safe Storage"). The master key is derived from this secret via PBKDF2:

| Parameter | Value |
|-----------|-------|
| Hash | SHA-1 |
| Salt | `saltysalt` |
| Iterations | 1003 |
| Key length | 16 bytes (AES-128) |

Decryption uses AES-128-CBC with a fixed IV of 16 space bytes (`0x20`). The ciphertext layout:

```
| v10   | AES-CBC ciphertext (PKCS5 padded) |
|-------|-------------------------------------|
| 3B    | remaining bytes                     |
```

There are three retrieval strategies, tried in order: (1) gcoredump exploit for securityd process memory, (2) direct keychain unlock with user's login password, (3) `security` CLI command (may trigger a GUI prompt). See [RFC-006](006-key-retrieval-mechanisms.md) for details.

## 4. Windows Encryption

Chromium on Windows stores a base64-encoded encrypted key in `Local State` at `os_crypt.encrypted_key`. The key recovery process is:

1. Base64-decode the `encrypted_key` value
2. Strip the 5-byte `DPAPI` ASCII prefix
3. Decrypt via Windows `CryptUnprotectData` (DPAPI) to obtain the 256-bit master key

With the master key, each encrypted value is decrypted as AES-256-GCM:

```
| v10   | nonce  | ciphertext + auth tag (16B) |
|-------|--------|-----------------------------|
| 3B    | 12B    | remaining bytes             |
```

**Legacy DPAPI** — values without a `v10`/`v20` prefix (pre-Chrome 80) are passed directly to `CryptUnprotectData`:

```
| DPAPI blob (no prefix)             |
|-------------------------------------|
| variable length                     |
```

## 5. Linux Encryption

Chromium on Linux retrieves a per-browser secret from D-Bus Secret Service (GNOME Keyring or KDE Wallet). The label matches the browser's storage name (e.g. "Chrome Safe Storage", "Chromium Safe Storage"). If D-Bus is unavailable, the hardcoded fallback password `peanuts` is used.

The master key is derived via PBKDF2 with different parameters than macOS:

| Parameter | Value |
|-----------|-------|
| Hash | SHA-1 |
| Salt | `saltysalt` |
| Iterations | 1 |
| Key length | 16 bytes (AES-128) |

Decryption uses the same AES-128-CBC scheme as macOS (fixed IV of 16 space bytes, PKCS5 padding).

## 6. v20 App-Bound Encryption (Chrome 127+)

Chrome 127 introduced App-Bound Encryption on Windows, identified by the `v20` prefix. This scheme binds the encryption key to the Chrome application identity, making it harder for external tools to decrypt. After decryption, the payload contains a 32-byte application header before the actual plaintext:

```
| v20   | nonce  | AES-GCM payload                    |
|-------|--------|-------------------------------------|
| 3B    | 12B    | remaining bytes                     |

After decryption:
| app-bound header | plaintext                          |
|------------------|------------------------------------|
| 32B              | remaining bytes                    |
```

**Current status**: v20 decryption is not yet implemented. Encountering a `v20`-prefixed value returns an error. This primarily affects recent Chrome installations on Windows.

## 7. Decryption Flow

The high-level decryption path for any encrypted Chromium value:

1. **Detect version** -- inspect the first 3 bytes of the ciphertext
2. **Route by version**:
   - `v10` -- strip prefix, call platform-specific decryption (AES-CBC on macOS/Linux, AES-GCM on Windows)
   - `v20` -- not yet supported, return error
   - DPAPI (no prefix) -- call Windows `CryptUnprotectData` directly (Windows only; returns error on other platforms)
3. **Return plaintext** -- the decrypted bytes are interpreted as a UTF-8 string

Each record is decrypted independently. A failure to decrypt one value does not prevent extraction of other records in the same database.

## Related RFCs

| RFC | Topic |
|-----|-------|
| [RFC-002](002-chromium-data-storage.md) | Chromium data file locations and storage formats |
| [RFC-006](006-key-retrieval-mechanisms.md) | Platform-specific master key retrieval |
