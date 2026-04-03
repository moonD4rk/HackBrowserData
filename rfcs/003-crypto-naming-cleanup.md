# RFC-003: Crypto Package and Naming Cleanup

**Author**: moonD4rk
**Status**: Proposed
**Created**: 2026-04-03

## Abstract

The `crypto/` package and cross-browser shared code have accumulated naming
and structural issues over time. This RFC tracks them for a future dedicated
refactoring pass. No code changes are proposed here.

## 1. crypto/asn1pbe.go

### Naming

| Current | Issue | Suggested |
|---------|-------|-----------|
| `nssPBE` | Too generic — "NSS" covers all Firefox crypto | `privateKeyPBE` — decrypts key4.db nssPrivate entries |
| `metaPBE` | "meta" is vague | `passwordCheckPBE` — decrypts key4.db metaData check |
| `loginPBE` | Acceptable but inconsistent | `credentialPBE` — decrypts logins.json credentials |
| `ASN1PBE` interface | Too technical for callers | `Decryptor` or `PBEDecryptor` |
| `SlatAttr` | **Typo** — should be `Salt` | `SaltAttr` |
| `AlgoAttr.Data.Data` | Nested names are meaningless | Flatten with descriptive field names |
| `AES128CBCDecrypt` | Misnomer — supports all AES key lengths | `AESCBCDecrypt` |

### Structure

`NewASN1PBE` uses trial-and-error `asn1.Unmarshal` to detect the type.
ASN1 parsing is lenient, so multiple structs may succeed. A safer approach
would be to parse the OID first, then unmarshal into the matching struct.

## 2. crypto/crypto_*.go

| Current | Issue |
|---------|-------|
| `DecryptWithChromium` | Platform-specific (AES-CBC on darwin, AES-GCM on windows) — name doesn't reflect this |
| `DecryptWithYandex` | Nearly identical to `DecryptWithChromium` on Windows |

## 3. Shared code between Chromium and Firefox

`discoverProfiles`, `hasAnySource`, `resolveSourcePaths`, `resolvedPath`
are nearly identical in both packages (~40 lines duplicated). Currently
each package keeps its own copy for independence. If more browser engines
are added (e.g. Safari WebKit), consider extracting to a shared package.

## 4. Priority

1. **SlatAttr typo** — trivial fix, do anytime
2. **AES128CBCDecrypt rename** — grep + rename, low risk
3. **ASN1PBE type/naming cleanup** — medium effort, needs comprehensive tests
4. **NewASN1PBE OID-first detection** — higher effort, must not break any Firefox version
5. **Shared profile discovery** — only when a third browser engine is added
