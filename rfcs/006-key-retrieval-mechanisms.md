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

- **`storage`** — platform-dependent identifier. On macOS it's the Keychain account label (e.g. `"Chrome"`); on Linux it's the D-Bus collection label (e.g. `"Chrome Safe Storage"`); on Windows it's the browser key used by `ABERetriever` to locate the elevation-service COM interface (e.g. `"chrome"`, `"edge"`). Ignored by `DPAPIRetriever`.
- **`localStatePath`** — path to `Local State` JSON file. Only used on Windows (DPAPI + ABE both read it).

The return value is the **ready-to-use decryption key** — either the raw AES key (Windows) or the PBKDF2-derived key (macOS/Linux).

`ChainRetriever` wraps multiple retrievers and tries them in order. The first successful result wins. If all fail, errors from every retriever are combined into a single error.

**Caching**: the retriever chain is created once per process inside `newPlatformInjector` (see `browser/browser_{darwin,linux,windows}.go`) and shared across every Chromium browser and every profile. macOS retrievers additionally use `sync.Once` internally, so multi-profile browsers only trigger one keychain prompt or memory dump.

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

Each browser identifies its Keychain entry with a short account string — typically the browser's base name (`"Chrome"`, `"Brave"`, `"Arc"`). Edge uses `"Microsoft Edge"`. Related variants share labels rather than defining their own: Chrome Beta aliases onto `"Chrome"`, Opera GX aliases onto `"Opera"`.

The authoritative mapping lives in the `Storage` field of each entry in `platformBrowsers()` (`browser/browser_darwin.go`).

## 4. Windows Key Retrieval

Chromium on Windows stores **two** master keys in `Local State` JSON: a legacy v10 key (`os_crypt.encrypted_key`, DPAPI-wrapped) and, since Chrome 127, an App-Bound Encryption v20 key (`os_crypt.app_bound_encrypted_key`, IElevator-wrapped). Both tiers can coexist on a single profile — Chrome 127+ encrypts *new* cookies with v20 but leaves pre-existing passwords and old cookies on v10 — so the retriever layer fetches both keys independently rather than via a ChainRetriever (see §4.4 and issue #578).

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

### 4.4 Dual-Tier Retrievers (V10 + V20)

Windows populates two slots of the `keyretriever.Retrievers` struct — V10 (legacy DPAPI) and V20 (Chrome 127+ App-Bound Encryption) — which run independently rather than as a first-success chain. V11 stays nil on Windows (Chromium does not emit v11 prefix there).

| Slot | Retriever | Source field | Mechanism |
|------|-----------|--------------|-----------|
| V10 | `DPAPIRetriever` | `os_crypt.encrypted_key` | `CryptUnprotectData` (Crypt32.dll) |
| V20 | `ABERetriever` | `os_crypt.app_bound_encrypted_key` | IElevator via reflective injection (see [RFC-010](010-chrome-abe-integration.md)) |

`browser/browser_windows.go::newPlatformInjector` calls `keyretriever.DefaultRetrievers()` and wires the resulting struct through `Browser.SetKeyRetrievers(r)`. At extract time `keyretriever.NewMasterKeys` runs each slot independently — a failure on one tier does not prevent the other from succeeding, because mixed-tier Chrome profiles (upgraded from pre-127) need partial success to be useful.

**Why not a ChainRetriever?** `ChainRetriever` has first-success semantics: once ABE returns a key, DPAPI is never called. That semantics is wrong for orthogonal tiers — it was the root cause of issue #578, where upgraded profiles' v10-encrypted passwords silently failed because only the v20 key was retrieved. `NewMasterKeys` evaluates each tier independently and returns an `errors.Join` of per-tier failures; log severity is a caller-side decision. `browser/chromium::getMasterKeys` currently logs all tier errors uniformly at `Warnf` — the distinction between "partial" and "total" failure was judged low-value for a short-lived CLI where all warn lines are visible in the default output.

**Non-ABE Chromium forks** (Opera, Vivaldi, Yandex, 360, QQ, Sogou) have `Storage: ""` in `platformBrowsers()`. `ABERetriever` returns `(nil, nil)` for empty storage, which `NewMasterKeys` treats silently as "not applicable" — so attempting ABE on these forks is a no-op, not a failure. Their V10 DPAPI key continues to work unchanged.

## 5. Linux Key Retrieval

### 5.1 Dual-Tier Retrievers (V10 + V11)

Linux populates two slots of the `keyretriever.Retrievers` struct — one per cipher prefix that Chromium emits on this platform:

| Slot | Prefix | Retriever | Mechanism | Chromium name |
|------|--------|-----------|-----------|---------------|
| V10 | `v10` | `PosixRetriever` | PBKDF2(`"peanuts"`) | kV10Key (matches upstream `PosixKeyProvider`) |
| V11 | `v11` | `DBusRetriever` | PBKDF2(D-Bus Secret Service password) | kV11Key (matches upstream `FreedesktopSecretKeyProvider`) |

V20 stays nil on Linux (App-Bound Encryption is Windows-only). v12 (Chromium's `SecretPortalKeyProvider`, Flatpak/xdg-desktop-portal) is a separate tier not yet implemented — see the `CipherV12` case in `decryptValue`.

**DBusRetriever** — queries the D-Bus Secret Service API (provided by `gnome-keyring-daemon` or `kwalletd`). Iterates all collections and items, looking for a label matching the browser's storage name. Populates the V11 slot because Chromium emits v11 prefix only when keyring access succeeds.

**PosixRetriever** — uses the hardcoded `"peanuts"` password that Chromium derives into a fixed 16-byte AES-128 key (kV10Key). Populates the V10 slot because Chromium emits v10 prefix for data encrypted with this key. Always succeeds deterministically.

### 5.2 Why Two Slots, Not a Chain

A profile can carry **both** v10 and v11 ciphertexts if the host has moved between keyring-equipped and headless sessions — e.g. a laptop that was once used in a headless shell then later in a full desktop session. The old `ChainRetriever{DBus, Fallback}` had first-success semantics: if D-Bus worked, peanuts was never called, leaving v10 ciphertexts undecryptable.

The split mirrors the Windows V10/V20 fix (§4.4) and the root-cause logic of issue #578: distinct cipher prefixes map to distinct key sources, so the retriever layer must produce both keys independently rather than picking "one winning" key.

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

Linux D-Bus labels follow a `"<name> Safe Storage"` convention, but many browsers alias onto a small shared set rather than defining their own. The three distinct labels are `"Chrome Safe Storage"`, `"Chromium Safe Storage"`, and `"Brave Safe Storage"` — everything else maps onto one of these.

The authoritative mapping lives in the `Storage` field of each entry in `platformBrowsers()` (`browser/browser_linux.go`).

## 6. Platform Summary

| Platform | Retrievers (slots populated) | PBKDF2 | Key Size |
|----------|------------------------------|:------:|----------|
| macOS | V10 = chain(Gcoredump → KeychainPassword* → SecurityCmd) | 1003 iterations | AES-128 |
| Windows | V10 = DPAPIRetriever; V20 = ABERetriever (Chrome 127+) | No | AES-256 |
| Linux | V10 = PosixRetriever ("peanuts" kV10Key); V11 = DBusRetriever (keyring kV11Key) | 1 iteration | AES-128 |

\* Only included when `--keychain-pw` is provided.

## 7. Safari Credential Extraction

Safari is **not** a consumer of the `KeyRetriever` interface. It has its own credential-extraction path in `browser/safari/extract_password.go`, which uses [keychainbreaker](https://github.com/moond4rk/keychainbreaker) directly to list `InternetPassword` records from `login.keychain-db`.

This is a deliberate architectural choice, not an oversight. The following sections explain why.

### 7.1 Why Safari Does Not Share the Chromium Chain

| Aspect | Chromium chain | Safari direct access |
|---|---|---|
| Output | A 16-byte AES-128 key | A list of `InternetPassword` records |
| Use case | Decrypt Login Data DB | Records *are* the credentials |
| Number of consumers | 10+ Chromium variants | 1 (Safari only) |
| Failure mode | Hard fail (no key → cannot decrypt) | Soft fail (degrade to metadata-only) |
| Caching benefit | High (multi-profile, multi-browser) | None (single browser, single call) |

Forcing Safari through the `KeyRetriever` interface would require returning a different type than `[]byte`, contradicting the interface's documented purpose as the *master-key* abstraction. Forcing it through a parallel "InternetPassword chain" would be over-engineering for a single consumer that has no fallback strategies worth chaining.

Note the "failure mode" row in particular: Chromium *must* have a master key or extraction fails entirely, so it needs a chain of escalating strategies. Safari can degrade gracefully — if the keychain cannot be unlocked, metadata-only export (URLs and usernames, no plaintext passwords) is still useful output, so a single "try keychainbreaker, warn on failure" is sufficient.

### 7.2 The General Rule

> **Each browser package owns its own credential-acquisition strategy. `crypto/keyretriever` exists only to share retrieval logic across the Chromium variant family. New browser implementations should follow Safari's and Firefox's example — own your credential code.**

Evidence the rule is already in force:

- **Firefox** (`browser/firefox/firefox.go`) does not import `keyretriever` or `keychainbreaker`. It derives keys from `key4.db` via internal NSS PBE. See RFC-005.
- **Safari** (`browser/safari/extract_password.go`) uses `keychainbreaker` directly for `InternetPassword` records.
- **Chromium variants** all go through `crypto/keyretriever` because they share exactly one chain and benefit from the shared `sync.Once` caching.

Future contributors adding a new macOS browser that reads credentials from the Keychain should add their access logic to that browser's package, not extend `keyretriever`. Only extend `keyretriever` if the new browser is a Chromium variant that fits the existing master-key chain.

### 7.3 Where the `--keychain-pw` Password Goes

The macOS login password is resolved once at startup by `browser/browser_darwin.go::resolveKeychainPassword`, then delivered to both consumers from within a single platform-specific closure, `newPlatformInjector` (defined per platform in `browser/browser_{darwin,linux,windows}.go`). The closure captures both the retriever chain and the raw password, and applies whichever capability interface each Browser happens to satisfy:

| Consumer | Capability interface | Defined in | Payload |
|---|---|---|---|
| Chromium browsers | `keyRetrieversSetter` | `browser/browser.go` | `keyretriever.Retrievers` struct (V10 / V11 / V20 slots; unused tiers nil) |
| Safari | `keychainPasswordSetter` | `browser/browser_darwin.go` | raw `string` |

The two setters are **intentionally not unified**. They carry different abstractions — one hands the browser a pre-assembled retrieval chain, the other hands the browser a credential token to unlock its own access path. Unifying them would create a leaky polymorphic interface with no real shared semantics. Note that `keychainPasswordSetter` is defined in the darwin-only file because Safari (its only implementer) is darwin-only.

`resolveKeychainPassword` additionally performs an early `TryUnlock` against `keychainbreaker` before the chain is built, so a bad password surfaces as a startup warning rather than a mid-extraction failure. The small cost of opening the keychain twice (once for validation, once inside `KeychainPasswordRetriever`) buys meaningful UX.

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
