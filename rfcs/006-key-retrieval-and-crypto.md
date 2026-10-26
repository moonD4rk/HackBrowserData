# RFC-006: Key Retrieval & Crypto

**Scope note.** This RFC fixes the _boundaries and contracts_ of the key-retrieval and decryption packages. It deliberately does not restate how any OS secret store is read or how any cipher is applied — those techniques are already documented in [RFC-014](014-chromium-encryption.md) (Chromium encryption), [RFC-015](015-key-retrieval-mechanisms.md) (key retrieval), [RFC-016](016-chrome-abe-integration.md) (Chrome ABE), [RFC-017](017-yandex-decryption.md) (Yandex), [RFC-019](019-firefox-encryption.md) (Firefox encryption), and [RFC-021](021-windows-locked-file-bypass.md) (Windows locked-file). Here each is simply a package with an interface.

## Two packages, one seam

- **`internal/masterkey`** owns everything that talks to an OS secret store: retrieving a Chromium user data dir's keys, one per version (DPAPI and ABE on Windows, Keychain on macOS, Secret Service on Linux), decrypting legacy per-value DPAPI blobs, and the portable `keys.json` dump. Platform code sits behind build tags; callers import one package. It knows nothing about records or browsers.
- **`internal/crypto`** holds pure primitives over bytes: AES-CBC and AES-GCM, 3DES, PBKDF2, and the ASN.1 PBE structures Firefox's NSS uses. It takes raw keys, nonces, and ciphertexts, makes no OS calls, and imports nothing from the project — which is what keeps it unit-testable with fixed vectors. It knows no blob format: Chromium's version prefix and nonce layout belong to the Chromium base, and Yandex's to its override.

`masterkey` calls `crypto`; `crypto` never sees a `masterkey.Set`. A decrypt that took a `Set` would have to live in one of the two packages and call back into the other — an import cycle — so dispatch lives with the format that defines it (below).

A third package sits beside them on macOS only: **`internal/keychain`** owns the user's login keychain — one handle per run, opened on first use, read by the Chromium `v10` retriever and the Safari password extractor. The login password, supplied by the CLI ([RFC-009](009-cli-surface.md)), reaches no other code.

## Key scopes

Different bases keep their secrets at different levels, so "once per user data dir" is not universal:

| Base     | Scope         | Where the secret comes from                                                                                  | Lazy handle                                                    |
| -------- | ------------- | ------------------------------------------------------------------------------------------------------------ | -------------------------------------------------------------- |
| Chromium | user data dir | `Local State` from the snapshot, plus the OS secret store — on macOS, that includes the run's login keychain | `ProfileSnapshot.MasterKeys`                                   |
| Firefox  | profile       | the profile's own `key4.db`, validated against its `logins.json` — no OS store, no prompt                    | derived inside the Firefox password extractor                  |
| Safari   | run           | the login keychain, opened once per run                                                                      | `keychain.Keychain`, captured by the Safari password extractor |

Two edge cases sit outside the table. Legacy Windows Chromium values without a `v10` prefix are raw DPAPI blobs, decrypted one by one through `masterkey.DecryptDPAPI` with the live user's DPAPI context — they never decrypt on another host. And on macOS the Chromium `v10` retriever and the Safari password extractor read the same login keychain, so the keychain is opened once per run and shared ([RFC-003](003-core-types.md); [RFC-009](009-cli-surface.md) defines how its password is supplied).

## `masterkey` contract

```go
type Version string         // "v10", "v11", "v20"
type Set map[Version][]byte // JSON form = today's keys.json "keys" object

type Retriever interface {
    Version() Version
    Retrieve(ctx context.Context, loc Locator) ([]byte, error)
}

// LoginKeychain is what the macOS v10 retriever reads from the run's login keychain; *keychain.Keychain satisfies it.
type LoginKeychain interface {
    GenericPassword(ctx context.Context, account string) ([]byte, error) // keychain.ErrLocked: no usable password
}

var ErrNotApplicable = errors.New("retriever not applicable")

func RetrieveAll(ctx context.Context, loc Locator) (Set, error)
func DecryptDPAPI(blob []byte) ([]byte, error) // Windows; ErrNotApplicable elsewhere
```

- `Locator` tells retrieval where one user data dir's keys are: the snapshot path of `Local State`, the `KeychainLabel`, the Windows `Exe` and `ABE` kind ([RFC-005](005-browser-definitions.md)), and on macOS the run's login keychain as a `LoginKeychain` — nil elsewhere, and in `restore`. The Chromium base builds it (`chromium.KeyLocator`), because only it knows the keys come from `Local State`; `app` adds the keychain. It never carries a password: only the keychain sees one ([RFC-009](009-cli-surface.md)).
- `RetrieveAll` runs this OS's retrievers in order. A retriever that doesn't apply returns `ErrNotApplicable` and is skipped. **Per version, the first key wins:** macOS's `v10` has three ordered strategies ([RFC-015](015-key-retrieval-mechanisms.md)), and once one succeeds the rest are not tried. A version's error is reported only when every retriever for it fails, so a strategy that failed before a later one succeeded is not an error.
- **Versions are independent, not a chain.** A Chrome profile upgraded past version 127 carries `v20` cookies beside `v10` passwords, so a `v20` failure keeps the `v10` key, as today. First-success across versions is the bug [RFC-015](015-key-retrieval-mechanisms.md) records as issue #578.
- **The macOS keychain retriever follows the handle.** With no keychain, or a locked one, it returns `ErrNotApplicable` and the next `v10` retriever runs — as today, where an empty or rejected password keeps that retriever out of the chain. The reason is reported once, through `Keychain.Err`, not once per browser. A label missing from an open keychain is an error, reported only if every `v10` retriever fails. The retriever that runs the `security` command asks the system service instead of reading the file, so it stays separate, with its own per-label cache.

### Laziness

`app` backs `ProfileSnapshot.MasterKeys` ([RFC-003](003-core-types.md)) with a per-user-data-dir call-once that asks the Chromium base for the dir's `Locator` and runs `RetrieveAll`; it keeps the retrieval error, which is reported once as the dir's `error` result, and every extraction in the dir gets whatever keys were found. The login keychain carries its own once: on macOS `app` builds it without opening it (a build-tagged one-liner), and its first query opens it and calls the `PasswordFunc` — the only step that can prompt. Go 1.20 has no `sync.OnceValues`, so each is a `sync.Once` with its stored result. The first extractor that needs cleartext triggers them; a run that only touches non-secret data never does, so it never prompts and never performs a privileged lookup. Today the macOS password is resolved as soon as any Chromium or Safari browser is discovered, and Chromium's `Extract` retrieves keys before it looks at the categories — so `dump -b chrome -c history` prompts. The lazy handles design that out.

## Decryption dispatch

Chromium's blob format belongs to the Chromium base, so its dispatch lives there. The version prefix picks the key from the `Set`: `v10`, `v11`, and `v20` decrypt with their version's key; `v12` is recognized and reported as unsupported, as today; a blob without a prefix goes to `masterkey.DecryptDPAPI`. The key length picks the cipher — 32 bytes means AES-GCM (Windows), 16 bytes means AES-CBC (macOS, Linux). Yandex's format (per-row AAD, a wrapped data key) is handled by its override with the same primitives.

Choosing the cipher from the key at runtime, rather than from `GOOS` at build time, is what makes **cross-host decryption** work: a `keys.json` and an archive taken on the same machine can be decrypted on any OS ([RFC-009](009-cli-surface.md)). The keys belong to the user data dir they were taken from; they do not decrypt another machine's data.

## Firefox and Safari

Firefox doesn't use an OS secret store the way Chromium does: its key is derived from the profile's own `key4.db` with the NSS primitives in `crypto`, inside `internal/browser/firefox`, and checked against `logins.json` to pick the right candidate. Everything it needs is in the snapshot.

Safari's passwords live in the login keychain. The Safari password extractor reads them through the run's `keychain.Keychain`, which `safari.Extractors` captures for the default profile only: the keychain belongs to the user, so attributing it to every profile would duplicate its entries. Without a usable password the keychain stays locked and the extractor exports metadata only, as today.

The keychain is not a snapshot source. It is not a browser file and lies outside every Chromium user data dir; it lives for the run, while a snapshot lives for one user data dir, so a copy in Safari's snapshot could not serve Chromium; and a copy would put one more sensitive file on disk. The handle reads the file once, when it opens — the stable bytes the snapshot rule is for.

## What stays out of these packages

- No data-type knowledge. `crypto` imports nothing from the project; `masterkey` never imports `browser` or `record`.
- No orchestration. Neither decides _which_ extractions run — that's the planner.
- No logging of secrets, ever. Retrieval returns errors; the reporter decides what the operator sees, and key bytes and passwords are never in a log line.

## Boundary with the technique RFCs

The format and technique RFCs (013–021) remain the source of truth for mechanics (how ABE injection works, how DPAPI and the keychain are read, the Yandex specifics, the locked-file bypass). This RFC only guarantees that those mechanics sit behind `masterkey` and `crypto` and nowhere else, so the rest of the program — and the people reading it on stage — deal with a handful of clean functions, not six techniques.
