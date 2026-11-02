# RFC-003: Secrets

## Boundaries

- **Secret retrieval** lives in one layer: talking to the OS stores (DPAPI and App-Bound Encryption on Windows, the Keychain on macOS, the Secret Service on Linux) and the portable keys file. It knows nothing about browsers or records.
- **Cryptography** is pure functions over bytes: no OS calls, no blob formats, testable with fixed vectors.
- **Blob formats** belong to each browser base, which decides which key decrypts what.
- **The macOS login keychain** is one resource for the whole run, opened at most once. Its consumers get the opened keychain, never the password.

## Scopes

Secrets live at different levels, so they are fetched at different levels: per data directory for Chromium's master keys, per profile for Firefox, whose key is derived from the profile's own files, and per run for the macOS login keychain.

## Lazy retrieval

A secret is fetched by the first record that needs it, once per scope, and then reused. A run that asks only for history or bookmarks never retrieves a key and never prompts. A retrieval failure is reported once for its scope; records that needed the secret are still written, with that field empty, and count as `partial`.

## Key versions

Chromium keys come in versions, and each version is retrieved independently: one failing never discards another. Within a version, the first strategy that works wins. The cipher is chosen from the key at run time rather than from the build's OS, so keys exported on one machine decrypt that machine's data on any OS.

## Never

- **No secret in a log line,** ever.
- **No new technique.** Only existing, published methods sit behind these boundaries.
- **No vendored loader.** The Windows payload is first-party code. The common third-party loader was rejected because it crashes on Windows internals it walks too far and depends on brittle structure layouts.
