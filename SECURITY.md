# Security Policy

HackBrowserData is a security research tool whose purpose is to decrypt and export browser data that the account running it can already access. This policy covers vulnerabilities in HackBrowserData itself — flaws that put the person running the tool, or the host it runs on, at risk.

## Supported Versions

| Version                            | Supported          |
| ---------------------------------- | ------------------ |
| Latest release                     | :white_check_mark: |
| Older releases (including all 0.x) | :x:                |

Security fixes land on `main` and ship in the next release; older releases are not patched.

## Reporting a Vulnerability

**Please do not report security vulnerabilities through public issues, discussions, or pull requests.**

Report them privately via [GitHub Security Advisories](https://github.com/moonD4rk/HackBrowserData/security/advisories/new) (Security tab → Report a vulnerability). If you cannot use GitHub, email [me@moond4rk.com](mailto:me@moond4rk.com) instead.

Please include as much of the following as you can:

- Affected version (output of `hack-browser-data version`) or commit
- Operating system and architecture, and the browser and its version if relevant
- The command and flags you ran, with step-by-step reproduction instructions
- A proof of concept (for example a crafted profile file, `keys.json`, or archive) and the impact you observed

## What to Expect

HackBrowserData is maintained in our spare time, so responses are best-effort:

- We aim to acknowledge your report within 7 days.
- We will let you know whether the report is accepted and keep you updated while a fix is prepared.
- Once a fixed release is out, we publish a GitHub Security Advisory (requesting a CVE where appropriate) and credit you, unless you prefer to stay anonymous.

Please keep the details private until the advisory is published.

## Scope

In scope — vulnerabilities in this repository's code, build, or releases, for example:

- Crafted input that HackBrowserData reads (browser profile files, a `keys.json`, or an archive passed to `restore`) leading to code execution, writes outside the output or temporary directory, or other compromise of the host running the tool
- Decrypted data or exported master keys leaking beyond their intended destination, such as files created with overly broad permissions, leftover temporary copies, or secrets written to logs
- Memory-safety bugs in the native Windows payload (`crypto/windows/abe_native/`)
- Weaknesses in the release pipeline (GitHub Actions workflows, release artifacts, the Homebrew tap) that could let someone tamper with what users install

Out of scope:

- HackBrowserData working as designed: decrypting and exporting browser data that the account running it can already read
- Antivirus or EDR products detecting or blocking the binary
- Vulnerabilities in browsers themselves or in the encryption schemes they rely on (such as DPAPI, the macOS Keychain, or Chrome's App-Bound Encryption) — please report these to the respective vendor
- Reports based only on Go toolchain or dependency versions without a demonstrated, reachable impact on HackBrowserData. The project intentionally builds with Go 1.20 to keep Windows 7 support.
- Binaries obtained anywhere other than the official channels below, including third-party forks and repackaged builds

## Official Downloads

Official binaries are published only on the [Releases](https://github.com/moonD4rk/HackBrowserData/releases) page and through the [`moonD4rk/homebrew-tap`](https://github.com/moonD4rk/homebrew-tap) Homebrew tap. Each release includes a `checksums-v<version>.txt` file; verify the SHA-256 of your download against it (for example with `sha256sum`, `shasum -a 256`, or PowerShell's `Get-FileHash`).
