# RFC-015: Key Retrieval Mechanisms

**Author**: moonD4rk\
**Status**: Living Document\
**Created**: 2026-04-05

## 1. Overview

Chromium-based browsers encrypt sensitive data (passwords, cookies, credit cards) using a **master key**. The master key is stored differently on each platform:

| Platform | Storage                              | Key Type                           |
| -------- | ------------------------------------ | ---------------------------------- |
| macOS    | macOS Keychain                       | Password string → PBKDF2 → AES-128 |
| Windows  | `Local State` JSON (DPAPI-encrypted) | Raw AES-256 key                    |
| Linux    | GNOME Keyring / KDE Wallet via D-Bus | Password string → PBKDF2 → AES-128 |

Each platform may have more than one retrieval strategy. A `Retriever` returns the key for one cipher version, reading what it needs from a `Locator`; `RetrieveAll` runs the platform's retrievers and collects one key per version.

For Chromium encryption details (cipher versions, AES-CBC/GCM), see [RFC-014](014-chromium-encryption.md). Firefox manages its own keys via `key4.db` — see [RFC-019](019-firefox-encryption.md).

## 2. Retriever Interface

```go
type Retriever interface {
    Version() Version // v10, v11, or v20
    Retrieve(ctx context.Context, loc Locator) ([]byte, error)
}

type Locator struct {
    LocalState    string        // the copied Local State; "" when the user data dir has none
    KeychainLabel string        // macOS Keychain account / Linux D-Bus Secret Service label
    Exe           string        // Windows: the program the ABE payload is injected into
    ExeFallbacks  []string      // Windows: programs tried when the registry has no App Paths entry
    ABE           ABEKind       // Windows: which elevation-service interface, or ABENone
    Keychain      LoginKeychain // macOS: the run's login keychain; nil elsewhere
}
```

Each retriever reads only the fields that apply to its platform; the others are ignored. The Chromium base builds the `Locator` for a user data dir (`chromium.KeyLocator`), copying `KeychainLabel`, `Exe`, `ExeFallbacks`, and `ABE` from the browser's `Platform` row ([RFC-005](005-browser-definitions.md)) and pointing `LocalState` at the snapshot's copy.

`Retrieve` returns the **ready-to-use decryption key** — either the raw AES key (Windows) or the PBKDF2-derived key (macOS/Linux).

`RetrieveAll(ctx, loc)` runs the platform's retrievers grouped by version: within a version the first success wins and the rest are skipped, a version whose every retriever fails contributes nothing, and an error is returned only when some version yielded no key. Versions are independent, so a `v20` failure never costs the `v10` key ([RFC-006](006-key-retrieval-and-crypto.md)).

**Scope**: `app` calls `RetrieveAll` once per user data dir, on the first extraction that needs cleartext, and shares the result across the dir's profiles; a run that touches only non-secret data never calls it. On macOS the `Locator` carries the run's one login-keychain handle, opened at most once ([RFC-006](006-key-retrieval-and-crypto.md)).

## 3. macOS Key Retrieval

Chromium on macOS stores the encryption password in the user's login keychain under a browser-specific account name (e.g. `"Chrome"`, `"Brave"`, `"Microsoft Edge"`).

### 3.1 Retrieval Strategies

**`gcoredumpRetriever`** — exploits **CVE-2025-24204** to extract keychain secrets from `securityd` process memory. Requires root. The exploit works because the `gcore` binary holds the `com.apple.system-task-ports.read` entitlement, bypassing TCC protections:

1. Find `securityd` PID via `sysctl`
2. Dump process memory via `gcore`
3. Parse heap regions via `vmmap`, scan `MALLOC_SMALL` regions for 24-byte key pattern
4. Try each candidate against `login.keychain-db`

**`keychainRetriever`** — unlocks `login.keychain-db` directly using the user's macOS login password (from `--keychain-pw` flag), powered by the [moond4rk/keychainbreaker](https://github.com/moond4rk/keychainbreaker) library which implements a full macOS Keychain file parser and decryptor in pure Go. Non-root, non-interactive.

**`securityRetriever`** — invokes `security find-generic-password -wa <label>`. Triggers a macOS password dialog. Last resort.

### 3.2 Retriever Order

| Priority | Strategy                   | Requires             | Interactive? |
| -------- | -------------------------- | -------------------- | :----------: |
| 1        | Gcoredump (CVE-2025-24204) | Root                 |      No      |
| 2        | Keychain password          | `--keychain-pw` flag |      No      |
| 3        | `security` CLI command     | Nothing              | Yes (dialog) |

### 3.3 PBKDF2 Derivation

All macOS strategies produce a raw password string from the keychain. This is derived into an AES-128 key via PBKDF2:

| Parameter  | Value              | Source                                                                                                                  |
| ---------- | ------------------ | ----------------------------------------------------------------------------------------------------------------------- |
| Salt       | `"saltysalt"`      | [os_crypt_mac.mm](https://source.chromium.org/chromium/chromium/src/+/master:components/os_crypt/os_crypt_mac.mm;l=157) |
| Iterations | 1003               |                                                                                                                         |
| Key length | 16 bytes (AES-128) |                                                                                                                         |
| Hash       | HMAC-SHA1          |                                                                                                                         |

### 3.4 Keychain Labels

Each browser identifies its Keychain entry with a short account string — typically the browser's base name (`"Chrome"`, `"Brave"`, `"Arc"`). Edge uses `"Microsoft Edge"`. Related variants share labels rather than defining their own: Chrome Beta aliases onto `"Chrome"`, Opera GX aliases onto `"Opera"`.

The authoritative mapping lives in the `KeychainLabel` field of each browser's `Platform` row ([RFC-005](005-browser-definitions.md)).

## 4. Windows Key Retrieval

Chromium on Windows stores **two** master keys in `Local State` JSON: a legacy v10 key (`os_crypt.encrypted_key`, DPAPI-wrapped) and, since Chrome 127, an App-Bound Encryption v20 key (`os_crypt.app_bound_encrypted_key`, IElevator-wrapped). Both tiers can coexist on a single profile — Chrome 127+ encrypts _new_ cookies with v20 but leaves pre-existing passwords and old cookies on v10 — so the retriever layer fetches both keys independently rather than as a first-success chain (see §4.4 and issue #578).

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

Unlike macOS/Linux, DPAPI gives the **final AES-256 key directly**. No intermediate password, no derivation step. The key is used as-is for AES-256-GCM decryption (see [RFC-014](014-chromium-encryption.md)).

### 4.4 Dual-Tier Retrievers (V10 + V20)

Windows registers two retrievers — V10 (legacy DPAPI) and V20 (Chrome 127+ App-Bound Encryption) — which `RetrieveAll` runs independently rather than as a first-success chain. No retriever claims V11 on Windows (Chromium does not emit the v11 prefix there).

| Version | Retriever        | Source field                       | Mechanism                                                                         |
| ------- | ---------------- | ---------------------------------- | --------------------------------------------------------------------------------- |
| V10     | `dpapiRetriever` | `os_crypt.encrypted_key`           | `CryptUnprotectData` (Crypt32.dll)                                                |
| V20     | `abeRetriever`   | `os_crypt.app_bound_encrypted_key` | IElevator via reflective injection (see [RFC-016](016-chrome-abe-integration.md)) |

`RetrieveAll` runs each version independently — a failure on one tier does not prevent the other from succeeding, because mixed-tier Chrome profiles (upgraded from pre-127) need partial success to be useful.

**Why versions are independent, not a first-success chain.** A chain across tiers stops at the first key: once ABE returns a v20 key, DPAPI is never called. That is wrong for orthogonal tiers — it was the root cause of issue #578, where upgraded profiles' v10-encrypted passwords silently failed because only the v20 key was retrieved. `RetrieveAll` groups retrievers by version, so each version returns its own key, and the per-version failures are joined into one error only when a version yields nothing ([RFC-006](006-key-retrieval-and-crypto.md)).

**Non-ABE Chromium forks** (Opera, Vivaldi, Yandex, 360, QQ, Sogou) set `ABE: ABENone` in their `Platform` row ([RFC-005](005-browser-definitions.md)), so `Locator.ABE` is `ABENone` and the V20 retriever returns `(nil, nil)` — `RetrieveAll` treats that as "not applicable", not a failure. Their V10 DPAPI key works unchanged.

## 5. Linux Key Retrieval

### 5.1 Dual-Tier Retrievers (V10 + V11)

Linux registers two retrievers — one per cipher prefix that Chromium emits on this platform:

| Version | Prefix | Retriever        | Mechanism                             | Chromium name                                             |
| ------- | ------ | ---------------- | ------------------------------------- | --------------------------------------------------------- |
| V10     | `v10`  | `posixRetriever` | PBKDF2(`"peanuts"`)                   | kV10Key (matches upstream `PosixKeyProvider`)             |
| V11     | `v11`  | `dbusRetriever`  | PBKDF2(D-Bus Secret Service password) | kV11Key (matches upstream `FreedesktopSecretKeyProvider`) |

No retriever claims V20 on Linux (App-Bound Encryption is Windows-only). v12 (Chromium's `SecretPortalKeyProvider`, Flatpak/xdg-desktop-portal) is a separate tier not yet implemented — see the `CipherV12` case in `decryptValue`.

**`dbusRetriever`** — queries the D-Bus Secret Service API (provided by `gnome-keyring-daemon` or `kwalletd`). Iterates all collections and items, looking for a label matching the browser's storage name. Claims V11 because Chromium emits the v11 prefix only when keyring access succeeds.

**`posixRetriever`** — uses the hardcoded `"peanuts"` password that Chromium derives into a fixed 16-byte AES-128 key (kV10Key). Claims V10 because Chromium emits the v10 prefix for data encrypted with this key. Always succeeds deterministically.

### 5.2 Why Two Versions, Not a Chain

A profile can carry **both** v10 and v11 ciphertexts if the host has moved between keyring-equipped and headless sessions — e.g. a laptop that was once used in a headless shell then later in a full desktop session. A first-success chain would stop at the first key: if D-Bus worked, peanuts was never tried, leaving v10 ciphertexts undecryptable. `RetrieveAll` keeps the versions independent, so both keys are fetched.

The split mirrors the Windows V10/V20 fix (§4.4) and the root-cause logic of issue #578: distinct cipher prefixes map to distinct key sources, so the retriever layer must produce both keys independently rather than picking "one winning" key.

### 5.3 PBKDF2 Derivation

Both strategies produce a password, derived via PBKDF2 with notably weaker parameters than macOS:

| Parameter  | Value              | Source                                                                                                                    |
| ---------- | ------------------ | ------------------------------------------------------------------------------------------------------------------------- |
| Salt       | `"saltysalt"`      | [os_crypt_linux.cc](https://source.chromium.org/chromium/chromium/src/+/main:components/os_crypt/os_crypt_linux.cc;l=100) |
| Iterations | **1**              |                                                                                                                           |
| Key length | 16 bytes (AES-128) |                                                                                                                           |
| Hash       | HMAC-SHA1          |                                                                                                                           |

A single iteration makes PBKDF2 essentially a keyed HMAC — no real key-stretching. Combined with the well-known fallback password `"peanuts"`, Linux Chromium encryption is trivial to break without the keyring.

### 5.4 Keychain Labels

Linux D-Bus labels follow a `"<name> Safe Storage"` convention, but many browsers alias onto a small shared set rather than defining their own. The three distinct labels are `"Chrome Safe Storage"`, `"Chromium Safe Storage"`, and `"Brave Safe Storage"` — everything else maps onto one of these.

The authoritative mapping lives in the `KeychainLabel` field of each browser's `Platform` row ([RFC-005](005-browser-definitions.md)).

## 6. Platform Summary

| Platform | Retrievers (slots populated)                                                    |     PBKDF2      | Key Size |
| -------- | ------------------------------------------------------------------------------- | :-------------: | -------- |
| macOS    | V10 = gcoredump → keychain* → security                                          | 1003 iterations | AES-128  |
| Windows  | V10 = dpapiRetriever; V20 = abeRetriever (Chrome 127+)                          |       No        | AES-256  |
| Linux    | V10 = posixRetriever ("peanuts" kV10Key); V11 = dbusRetriever (keyring kV11Key) |   1 iteration   | AES-128  |

\* Only included when a non-empty password resolves — either via `--keychain-pw` flag or an interactive TTY prompt.

## 7. Safari Credential Extraction

Safari is **not** a consumer of the `Retriever` interface. It has its own credential-extraction path in `internal/browser/safari/extract_password.go`, which uses [keychainbreaker](https://github.com/moond4rk/keychainbreaker) directly to list `InternetPassword` records from `login.keychain-db`.

This is a deliberate architectural choice, not an oversight. The following sections explain why.

### 7.1 Why Safari Does Not Share the Chromium Chain

| Aspect              | Chromium chain                      | Safari direct access                 |
| ------------------- | ----------------------------------- | ------------------------------------ |
| Output              | A 16-byte AES-128 key               | A list of `InternetPassword` records |
| Use case            | Decrypt Login Data DB               | Records _are_ the credentials        |
| Number of consumers | 10+ Chromium variants               | 1 (Safari only)                      |
| Failure mode        | Hard fail (no key → cannot decrypt) | Soft fail (degrade to metadata-only) |
| Caching benefit     | High (multi-profile, multi-browser) | None (single browser, single call)   |

Forcing Safari through the `Retriever` interface would require returning a different type than `[]byte`, contradicting the interface's documented purpose as the _master-key_ abstraction. Forcing it through a parallel "InternetPassword chain" would be over-engineering for a single consumer that has no fallback strategies worth chaining.

Note the "failure mode" row in particular: Chromium _must_ have a master key or extraction fails entirely, so it needs a chain of escalating strategies. Safari can degrade gracefully — if the keychain cannot be unlocked, metadata-only export (URLs and usernames, no plaintext passwords) is still useful output, so a single "try keychainbreaker, warn on failure" is sufficient.

### 7.2 The General Rule

> **Each browser package owns its own credential-acquisition strategy. `masterkey` exists only to share retrieval logic across the Chromium variant family. New browser implementations should follow Safari's and Firefox's example — own your credential code.**

Evidence the rule is already in force:

- **Firefox** (`internal/browser/firefox/firefox.go`) does not import `masterkey` or `keychainbreaker`. It derives keys from `key4.db` via internal NSS PBE. See RFC-019.
- **Safari** (`internal/browser/safari/extract_password.go`) uses `keychainbreaker` directly for `InternetPassword` records.
- **Chromium variants** all go through `masterkey` because they share one retrieval path and the per-user-data-dir caching ([RFC-006](006-key-retrieval-and-crypto.md)).

Future contributors adding a new macOS browser that reads credentials from the Keychain should add their access logic to that browser's package, not extend `masterkey`. Only extend `masterkey` if the new browser is a Chromium variant that fits the existing per-version retrievers.

### 7.3 Where the `--keychain-pw` Password Goes

Validate turns the chosen input (`--keychain-pw`, `--keychain-pw-stdin`, a terminal prompt, or none) into one `keychain.PasswordFunc` ([RFC-009](009-cli-surface.md)). On macOS `app` builds the run's single login-keychain handle with it; the handle opens lazily, at most once, and the `PasswordFunc` runs only then ([RFC-006](006-key-retrieval-and-crypto.md)). Two consumers read through that one handle, each declaring the single method it needs:

| Consumer         | Interface                 | Reaches it through                         |
| ---------------- | ------------------------- | ------------------------------------------ |
| Chromium `v10`   | `masterkey.LoginKeychain` | `Locator.Keychain`                         |
| Safari passwords | `safari.LoginKeychain`    | the extractor built by `safari.Extractors` |

Both interfaces name the same thing — the open-or-locked keychain — under the same method, so neither consumer sees a password string: an opened keychain carries its own answer, open or locked with a reason ([RFC-004](004-naming-decisions.md)).

Opening the handle is itself the password check: a missing or wrong password leaves it locked, `Keychain.Err` gives the reason, and the reporter prints it once ([RFC-006](006-key-retrieval-and-crypto.md)). There is no separate validation pass, and the keychain opens at most once per run — a run that reads only non-secret data never opens it at all.

## References

- **macOS**: [os_crypt_mac.mm](https://source.chromium.org/chromium/chromium/src/+/master:components/os_crypt/os_crypt_mac.mm;l=157)
- **Windows**: [os_crypt_win.cc](https://source.chromium.org/chromium/chromium/src/+/main:components/os_crypt/os_crypt_win.cc)
- **Linux**: [os_crypt_linux.cc](https://source.chromium.org/chromium/chromium/src/+/main:components/os_crypt/os_crypt_linux.cc;l=100)
- **CVE-2025-24204**: [Exploit PoC](https://github.com/FFRI/CVE-2025-24204/tree/main/decrypt-keychain), [Apple advisory](https://support.apple.com/en-us/122373)
- **DPAPI**: [CryptUnprotectData](https://learn.microsoft.com/en-us/windows/win32/api/dpapi/nf-dpapi-cryptunprotectdata)

## Related RFCs

| RFC                                   | Topic                                       |
| ------------------------------------- | ------------------------------------------- |
| [RFC-014](014-chromium-encryption.md) | Chromium encryption mechanisms per platform |
| [RFC-019](019-firefox-encryption.md)  | Firefox NSS encryption and key derivation   |
