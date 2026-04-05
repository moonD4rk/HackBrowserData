# RFC-006: Key Retrieval Mechanisms

**Author**: moonD4rk
**Status**: Living Document
**Created**: 2026-04-05

## 1. Overview

Chromium-based browsers encrypt sensitive data (passwords, cookies, credit cards) using a **master key**. The master key is stored differently on each platform:

| Platform | Storage | Key Type |
|----------|---------|----------|
| macOS | macOS Keychain | Password string → PBKDF2 → AES-128 |
| Windows | `Local State` JSON (DPAPI-encrypted) | Raw AES-256 key |
| Linux | GNOME Keyring / KDE Wallet via D-Bus | Password string → PBKDF2 → AES-128 |

Each platform may have multiple retrieval strategies. The `KeyRetriever` interface and `ChainRetriever` pattern abstract over these strategies, trying each in priority order until one succeeds.

For Chromium encryption details (cipher versions, AES-CBC/GCM), see [RFC-003](003-chromium-encryption.md). Firefox manages its own keys via `key4.db` — see [RFC-005](005-firefox-encryption.md).

## 2. KeyRetriever Interface

The interface takes two parameters:

- **`storage`** — keychain/keyring label identifying the browser's secret (e.g. `"Chrome"` on macOS, `"Chrome Safe Storage"` on Linux). Unused on Windows.
- **`localStatePath`** — path to `Local State` JSON file. Only used on Windows.

The return value is the **ready-to-use decryption key** — either the raw AES key (Windows) or the PBKDF2-derived key (macOS/Linux).

`ChainRetriever` wraps multiple retrievers and tries them in order. The first successful result wins. If all fail, errors from every retriever are combined into a single error.

**Caching**: the retriever is created once per browser and shared across all profiles. macOS retrievers use `sync.Once` internally, so multi-profile browsers only trigger one keychain prompt or memory dump.

## 3. macOS Key Retrieval

Chromium on macOS stores the encryption password in the user's login keychain under a browser-specific account name (e.g. `"Chrome"`, `"Brave"`, `"Microsoft Edge"`).

### 3.1 Retrieval Strategies

**GcoredumpRetriever** — exploits **CVE-2025-24204** to extract keychain secrets from `securityd` process memory. Requires root. The exploit works because the `gcore` binary holds the `com.apple.system-task-ports.read` entitlement, bypassing TCC protections:

1. Find `securityd` PID via `sysctl`
2. Dump process memory via `gcore`
3. Parse heap regions via `vmmap`, scan `MALLOC_SMALL` regions for 24-byte key pattern
4. Try each candidate against `login.keychain-db`

**KeychainPasswordRetriever** — unlocks `login.keychain-db` directly using the user's macOS login password (from `--keychain-pw` flag), powered by the [moond4rk/keychainbreaker](https://github.com/moond4rk/keychainbreaker) library which implements a full macOS Keychain file parser and decryptor in pure Go. Non-root, non-interactive.

**SecurityCmdRetriever** — invokes `security find-generic-password -wa <label>`. Triggers a macOS password dialog. Last resort.

### 3.2 Chain Order

| Priority | Strategy | Requires | Interactive? |
|----------|----------|----------|:------------:|
| 1 | Gcoredump (CVE-2025-24204) | Root | No |
| 2 | Keychain password | `--keychain-pw` flag | No |
| 3 | `security` CLI command | Nothing | Yes (dialog) |

### 3.3 PBKDF2 Derivation

All macOS strategies produce a raw password string from the keychain. This is derived into an AES-128 key via PBKDF2:

| Parameter | Value | Source |
|-----------|-------|--------|
| Salt | `"saltysalt"` | [os_crypt_mac.mm](https://source.chromium.org/chromium/chromium/src/+/master:components/os_crypt/os_crypt_mac.mm;l=157) |
| Iterations | 1003 | |
| Key length | 16 bytes (AES-128) | |
| Hash | HMAC-SHA1 | |

### 3.4 Storage Labels

| Browser | Keychain Account |
|---------|-----------------|
| Chrome / Chrome Beta | `"Chrome"` |
| Edge | `"Microsoft Edge"` |
| Chromium | `"Chromium"` |
| Opera / OperaGX | `"Opera"` |
| Vivaldi | `"Vivaldi"` |
| Brave | `"Brave"` |
| Yandex | `"Yandex"` |
| Arc | `"Arc"` |
| CocCoc | `"CocCoc"` |

## 4. Windows Key Retrieval

Chromium on Windows stores the master key in `Local State` JSON, encrypted with DPAPI.

### 4.1 DPAPI Background

Windows Data Protection API (DPAPI) is a built-in symmetric encryption service provided by `Crypt32.dll`. It uses the logged-in user's Windows credentials (derived from the user's login password) as the root key material. Applications call `CryptProtectData` to encrypt and `CryptUnprotectData` to decrypt, without needing to manage keys themselves.

Key characteristics:
- **User-scoped** — data encrypted by one Windows user cannot be decrypted by another user, even on the same machine
- **Machine-bound** — the encrypted blob cannot be decrypted on a different machine (unless roaming credentials are used)
- **No password prompt** — decryption is transparent to the calling process as long as it runs under the correct user session

### 4.2 Retrieval Flow

```
Local State → os_crypt.encrypted_key (base64 string)

| "DPAPI" prefix | DPAPI-encrypted AES key  |
|----------------|--------------------------|
| 5B (ASCII)     | remaining bytes          |

  → strip prefix
  → CryptUnprotectData (Crypt32.dll)
  → 32-byte AES-256 master key
```

The implementation loads `Crypt32.dll` at runtime via `syscall.NewLazyDLL` and calls `CryptUnprotectData` with a `DATA_BLOB` structure pointing to the ciphertext. Windows internally derives the decryption key from the user's credentials and returns the plaintext master key.

### 4.3 No PBKDF2 Needed

Unlike macOS/Linux, DPAPI gives the **final AES-256 key directly**. No intermediate password, no derivation step. The key is used as-is for AES-256-GCM decryption (see [RFC-003](003-chromium-encryption.md)).

### 4.4 Single Retriever

Windows uses only `DPAPIRetriever` — no chain needed. Both `storage` and `keychainPassword` parameters are ignored.

## 5. Linux Key Retrieval

### 5.1 Retrieval Strategies

**DBusRetriever** — queries the D-Bus Secret Service API (provided by `gnome-keyring-daemon` or `kwalletd`). Iterates all collections and items, looking for a label matching the browser's storage name.

**FallbackRetriever** — when D-Bus is unavailable (headless servers, Docker, CI), uses the hardcoded password `"peanuts"`. This matches Chromium's own fallback behavior.

### 5.2 Chain Order

| Priority | Strategy | Requires | Interactive? |
|----------|----------|----------|:------------:|
| 1 | D-Bus Secret Service | D-Bus session + keyring | No |
| 2 | Fallback (`"peanuts"`) | Nothing | No |

### 5.3 PBKDF2 Derivation

Both strategies produce a password, derived via PBKDF2 with notably weaker parameters than macOS:

| Parameter | Value | Source |
|-----------|-------|--------|
| Salt | `"saltysalt"` | [os_crypt_linux.cc](https://source.chromium.org/chromium/chromium/src/+/main:components/os_crypt/os_crypt_linux.cc;l=100) |
| Iterations | **1** | |
| Key length | 16 bytes (AES-128) | |
| Hash | HMAC-SHA1 | |

A single iteration makes PBKDF2 essentially a keyed HMAC — no real key-stretching. Combined with the well-known fallback password `"peanuts"`, Linux Chromium encryption is trivial to break without the keyring.

### 5.4 Storage Labels

| Browser | D-Bus Label |
|---------|-------------|
| Chrome / Chrome Beta / Vivaldi | `"Chrome Safe Storage"` |
| Chromium / Edge / Opera | `"Chromium Safe Storage"` |
| Brave | `"Brave Safe Storage"` |

## 6. Platform Summary

| Platform | Chain | PBKDF2 | Key Size |
|----------|-------|:------:|----------|
| macOS | Gcoredump → KeychainPassword* → SecurityCmd | 1003 iterations | AES-128 |
| Windows | DPAPI only | No | AES-256 |
| Linux | DBus → Fallback | 1 iteration | AES-128 |

\* Only included when `--keychain-pw` is provided.

## References

- **macOS**: [os_crypt_mac.mm](https://source.chromium.org/chromium/chromium/src/+/master:components/os_crypt/os_crypt_mac.mm;l=157)
- **Windows**: [os_crypt_win.cc](https://source.chromium.org/chromium/chromium/src/+/main:components/os_crypt/os_crypt_win.cc)
- **Linux**: [os_crypt_linux.cc](https://source.chromium.org/chromium/chromium/src/+/main:components/os_crypt/os_crypt_linux.cc;l=100)
- **CVE-2025-24204**: [Exploit PoC](https://github.com/FFRI/CVE-2025-24204/tree/main/decrypt-keychain), [Apple advisory](https://support.apple.com/en-us/122373)
- **DPAPI**: [CryptUnprotectData](https://learn.microsoft.com/en-us/windows/win32/api/dpapi/nf-dpapi-cryptunprotectdata)

## Related RFCs

| RFC | Topic |
|-----|-------|
| [RFC-003](003-chromium-encryption.md) | Chromium encryption mechanisms per platform |
| [RFC-005](005-firefox-encryption.md) | Firefox NSS encryption and key derivation |
