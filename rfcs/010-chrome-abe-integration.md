# RFC-010: Chrome App-Bound Encryption Integration

**Author**: moonD4rk
**Status**: Living Document
**Created**: 2026-04-17
**Last updated**: 2026-04-19

## 1. Overview

Chrome 127+ introduced **App-Bound Encryption (ABE)** on Windows. The `Local State` key that decrypts `v10`-era cookies/passwords is no longer a user-bound DPAPI blob; it is now an *app-bound* blob that only a legitimate `chrome.exe` / `msedge.exe` / `brave.exe` process can unwrap via the `elevation_service` COM RPC (`IElevator::DecryptData`).

This RFC documents how HackBrowserData integrates ABE support end-to-end while keeping the project **pure Go by default, cross-platform, zero disk footprint at runtime, and zero cost for non-Windows contributors.**

Related RFCs:

- [RFC-003](003-chromium-encryption.md) — cipher versions (v10, v11, v20)
- [RFC-006](006-key-retrieval-mechanisms.md) — `KeyRetriever` / `ChainRetriever`
- [RFC-009](009-windows-locked-file-bypass.md) — other Windows-specific handling

### 1.1 Tested matrix (as of 2026-04-19)

Single source of truth for version pins and observed-working targets. When re-validating, update dates and re-run the regression flow documented in the author's private playbook (not in this RFC).

| Component | Contract | Last verified |
|---|---|---|
| Go toolchain | **1.20** (pinned; Go 1.21+ drops Win7) | 1.20.14 |
| Windows host | Any Win10 1909+ (PE loader + UCRT) | Windows 10 19044 |
| Chrome family | Any v127+ (ABE introduced) | Chrome 147.0.7727.57 |
| zig toolchain | 0.13+ (for `make payload`) | 0.16.0 |
| Target arch | x86_64 only (x86 / ARM64 reserved) | x86_64 |

## 2. The constraint that shapes the design

`elevation_service` verifies the caller:

1. The calling process's main executable must be a **legitimate browser binary** (path in `Program Files`, signed by the browser vendor).
2. Process integrity is checked via other sandbox gates.

Consequence: **the code that issues the `IElevator::DecryptData` COM call must be running inside a `chrome.exe` / `msedge.exe` / `brave.exe` process**. A plain Go process, even elevated, is refused.

The architecture therefore ships a small native payload, injects it into a freshly-spawned browser process, has it invoke the COM RPC, and hands the 32-byte master key back to the Go side. Everything else (v20 AES-GCM decrypt, DB iteration, JSON output) is already Go.

## 3. Architecture

End-to-end flow when `hack-browser-data.exe` encounters a v20 Chromium cookie on Windows:

**Stage 1 — Our process** (`hbd.exe`, `CGO_ENABLED=0`)

```
browser/chromium.Extract()
  → keyretriever.Chain [ABERetriever, DPAPIRetriever]
  → ABERetriever.RetrieveKey():
      reads Local State → extracts APPB-prefixed blob
      resolves browser exe via registry App Paths
  → utils/injector.Reflective.Inject(exePath, payload, env)
```

**Stage 2 — Payload preparation** (still our process)

1. Read the embedded payload via `//go:embed abe_extractor_amd64.bin` (~75 KB).
2. Patch 5 × `uintptr` function pointers into the payload's DOS stub (see §4.4).
3. Look up `Bootstrap`'s **raw file offset** (not RVA) via `debug/pe`.

**Stage 3 — Spawn + inject** (still our process, target is newly spawned)

```
CreateProcessW(browser.exe, CREATE_SUSPENDED)
VirtualAllocEx(target, RWX, sizeOf(payload))
WriteProcessMemory(patched bytes)
ResumeThread(mainThread) + Sleep(500ms)      // let ntdll finish loader init
CreateRemoteThread(target, remoteBase + bootstrapFileOffset)
```

**Stage 4 — Inside the remote `browser.exe`**

The hijacked thread runs `Bootstrap` (C), our self-written reflective DLL loader. On return it calls the payload's `DllMain`:

```
Bootstrap                     → see §4.1 (7 helpers + orchestrator)
  ↓ calls DllMain(DLL_PROCESS_ATTACH, imageBase)
DoExtractKey                  → see §4.2
  CoCreateInstance(CLSID, IID_v2 | fallback IID_v1)
  CoSetProxyBlanket(PKT_PRIVACY + IMPERSONATE)
  vtbl[slot]->DecryptData(bstrEnc)
    ↓ COM RPC
  elevation_service (SYSTEM) → returns 32-byte plaintext key
  publish_key()  → imageBase[0x40..0x5F]  (success)
  publish_error(code, hr, comErr)         (failure)
```

**Stage 5 — Back in our process**

1. `WaitForSingleObject(thread, 30s)` — covers cold-start of `GoogleChromeElevationService`.
2. `ReadProcessMemory` for the 12-byte diagnostic header, then 32-byte key when `status == ready`.
3. `TerminateProcess(browser)` — the target was a throwaway from the start.

The returned key flows back up to `crypto.DecryptChromiumV20` (cross-platform AES-256-GCM; see §5.3) and then to the usual cookie/password extraction pipeline.

## 4. C payload — `crypto/windows/abe_native/`

Three translation units, ~500 lines of pure C. No C++, no assembly, no direct syscalls, no vendored third-party code (Stephen Fewer's loader was evaluated and rejected — see §8.2). Built with `zig cc -target x86_64-windows-gnu`.

### 4.1 Reflective loader — `bootstrap.c`

`Bootstrap(LPVOID lpParameter)` exported as `__declspec(dllexport)`. The Go injector calls it at its **raw file offset** (not RVA) because we inject raw file bytes rather than a mapped image.

Structure after refactor: **one ~30-line orchestrator + seven single-purpose static helpers**:

| Helper | Responsibility |
|---|---|
| `locate_own_image_base` | Backward-scan from `__builtin_return_address(0)` for MZ/PE magic (must stay `noinline`) |
| `read_preresolved_imports` | Read 5 function pointers the Go injector patched into DOS stub (§4.4) |
| `allocate_and_copy_image` | `VirtualAlloc(SizeOfImage, RW)` + copy headers/sections |
| `apply_base_relocations` | Walk `IMAGE_DIRECTORY_ENTRY_BASERELOC`, fix `IMAGE_REL_BASED_DIR64` |
| `link_iat` | Resolve each imported DLL + fill IAT via pre-resolved `LoadLibraryA` / `GetProcAddress` |
| `set_section_protections` | `.text → RX`, `.rdata → R`, `.data → RW` per `Characteristics` |
| `invoke_dllmain` | Call mapped `DllMain(DLL_PROCESS_ATTACH, imageBase)` — `imageBase` is the scratch handoff pointer |

Progress markers: after each major step the orchestrator writes one byte to `imageBase + BOOTSTRAP_MARKER_OFFSET` (0x28, inside `IMAGE_DOS_HEADER.e_res2`). The Go injector reads this back on failure to pinpoint the stage.

### 4.2 COM extractor — `abe_extractor.c`

Standard DLL whose `DllMain(DLL_PROCESS_ATTACH)` delegates to `DoExtractKey`, which is itself a thin orchestrator:

```
DoExtractKey(imageBase)
  CoInitializeEx(APARTMENTTHREADED)
  GetOwnExeBasename → LookupBrowserByExe (com_iid.c)
  extract_key_inner(ids) → extract_result { hr, comErr, errCode, plain }
  if errCode == OK && plain correct length:
      publish_key(imageBase, plain)       // atomic write with MemoryBarrier
  else:
      publish_error(imageBase, code, hr, comErr)
  SysFreeString + SecureZeroMemory + CoUninitialize
```

`extract_key_inner` owns a single resource (`bstrEnc`) and uses early returns — no goto chain. Steps: read `HBD_ABE_ENC_B64` env var, base64-decode, `SysAllocStringByteLen`, `CoCreateInstance(IID_v2)` with fallback to `IID_v1`, `CoSetProxyBlanket(PKT_PRIVACY + IMPERSONATE)`, **slot-based vtable dispatch** of `DecryptData` (slot 5 for Chrome-family, 8 for Edge, 13 for Avast).

**Diagnostic channel** (`extract_err_code` / `hresult` / `com_err` fields in the scratch region, added alongside the success byte): lets the Go side report structured failures like `err=CoCreateInstance failed, hr=E_ACCESSDENIED (0x80070005), comErr=0x0` instead of the old `status=0x00, marker=0xff`. Failure categories enumerated in `bootstrap_layout.h`:

```
ABE_ERR_BASENAME / BROWSER_UNKNOWN / ENV_MISSING / BASE64
ABE_ERR_BSTR_ALLOC / COM_CREATE / DECRYPT_DATA / KEY_LEN
```

### 4.3 Vendor table — `com_iid.c` / `com_iid.h`

Static table mapping `exe_basename → { CLSID, IID_v1, IID_v2, kind }`. `kind` selects the DecryptData vtable slot. Schema:

```c
{ "chrome.exe", CHROME_BASE, { CLSID_bytes }, { IID_v1_bytes }, TRUE, { IID_v2_bytes } }
```

Current coverage: Chrome Stable/Beta, Brave, Edge, Avast Secure Browser, CocCoc. Source file `crypto/windows/abe_native/com_iid.c` is the authoritative list — see §10 for how to add a new fork.

### 4.4 Pre-resolved imports (non-obvious design)

The original plan had `Bootstrap` walk the PEB's `InMemoryOrderModuleList` to find kernel32 / ntdll and resolve `LoadLibraryA` etc. via export-table parsing. It worked in test processes but **crashed reproducibly in Chrome 147's broker process** — `resolve_export` returned NULL for every LDR entry. Root cause was never fully pinpointed (Chrome-specific process state + Windows 10 LDR layout interaction).

Workaround: **Go resolves the 5 required functions in its own process** (via `windows.LazyProc.Addr()` in `utils/injector/winapi_windows.go`) and **patches the raw u64 values into the payload's DOS stub** at fixed offsets before `WriteProcessMemory`. `Bootstrap` just reads them; no PEB walk, no export parsing.

Validity relies on Windows **KnownDlls + session-consistent ASLR** — `kernel32.dll` and `ntdll.dll` load at the same virtual address in all processes of a boot session.

## 5. Go integration

### 5.1 Injector package — `utils/injector/`

Three files collaborate:

| File | Role |
|---|---|
| `reflective_windows.go` | `Reflective.Inject(exePath, payload, env) ([]byte, error)` — the orchestrator |
| `winapi_windows.go` | Package-level `windows.LazyProc` handles + `callBoolErr` helper. Centralizes `VirtualAllocEx` / `CreateRemoteThread` / NtFlushIC / import-address lookups. `ReadProcessMemory` / `WriteProcessMemory` use `x/sys/windows` typed wrappers directly. |
| `errors_windows.go` | `formatABEError(scratchResult) string` — renders the C-side diag channel into human-readable strings via two lookup maps (`ABE_ERR_*` names + known HRESULT names like `E_ACCESSDENIED`). |
| `pe_windows.go` | `FindExportFileOffset(dllBytes, "Bootstrap")` — raw-file offset via `debug/pe`. |
| `arch_windows.go` | Architecture validation (amd64-only today). |

`scratchResult` is the Go mirror of the remote process's 12-byte diagnostic header: `Marker / Status / ErrCode / HResult / ComErr` + optional 32-byte `Key`. One `ReadProcessMemory` covers the header; a second reads the key only when `Status == KeyStatusReady`.

### 5.2 Scratch layout codegen

The C payload and Go injector communicate through a byte-level protocol inside the target process's DOS stub region. The layout is defined **once** as a `BootstrapScratch` struct + `offsetof`-based macros in `crypto/windows/abe_native/bootstrap_layout.h`. `_Static_assert`s in the same header guarantee compile-time detection of layout drift:

```c
_Static_assert(offsetof(struct BootstrapScratch, marker) == 0x28, "marker offset");
_Static_assert(offsetof(struct BootstrapScratch, hresult) == 0x2C, "hresult offset");
_Static_assert(offsetof(struct BootstrapScratch, shared) == 0x40, "shared offset");
```

Go consumes the same constants via **`go tool cgo -godefs`** (a development-time tool, not a runtime dependency). `make gen-layout` regenerates `crypto/windows/abe_native/bootstrap/layout.go` from `bootstrap_layout.h` using `CC="zig cc"` for bit-identical results across host OSes. `make gen-layout-verify` is wired into CI to fail if the committed `layout.go` is stale.

**Why `cgo -godefs` rather than runtime `import "C"`**: we only need constants shared, not FFI to C functions. Runtime CGO would force the whole project into `CGO_ENABLED=1`, losing the "non-Windows contributor needs no C toolchain" guarantee. `cgo -godefs` bakes the values into a pure-Go file that commits to git; the project stays `CGO_ENABLED=0`.

### 5.3 Retriever wiring & v20 routing

`keyretriever.DefaultRetrievers()` on Windows returns a `Retrievers` struct with `V10 = &DPAPIRetriever{}` and `V20 = &ABERetriever{}`. The two tiers are wired independently — not in a ChainRetriever — because a single Chrome profile upgraded from pre-127 can carry mixed v10+v20 ciphertexts, and both keys must be available for `decryptValue` to route each ciphertext to its matching tier (see [RFC-006](006-key-retrieval-mechanisms.md) §4.4 and issue #578). `ABERetriever.RetrieveKey`:

1. Reads `Local State` → extracts `os_crypt.app_bound_encrypted_key` → strips `APPB` prefix. If the field is missing, `ABERetriever` returns `(nil, nil)`, `V20` remains empty, and the independently-wired `V10` DPAPI tier still runs.
2. Resolves browser executable via `utils/winutil/browser_path_windows.go` (registry App Paths → hardcoded fallback).
3. Base64-encodes the encrypted blob and passes it as `HBD_ABE_ENC_B64` env var.
4. `Reflective.Inject(exePath, payload, env)` runs the full flow in §3.
5. Returns the 32-byte key on success, or a formatted diagnostic error.

On extraction success, logs at `Info` level (`abe: retrieved <browser> master key via reflective injection`).

**v20 decryption** is cross-platform by design: `browser/chromium/decrypt.go` routes `CipherV20` → `crypto.DecryptChromiumV20` (defined in `crypto/crypto.go`, uses `AESGCMDecrypt`). This lets Linux/macOS CI exercise the same decryption path as Windows — only the key-source side is platform-gated.

## 6. Build chain

- **Default build** (any host, no zig): `go build ./cmd/hack-browser-data/` succeeds; ABE is stubbed out. Legacy v10/v11 cookies still decrypt via DPAPI.
- **Windows release with ABE**: `make build-windows` = `make payload` (zig cc → `crypto/abe_extractor_amd64.bin`) + `GOOS=windows go build -tags abe_embed`. The `abe_embed` tag activates `//go:embed` on the compiled binary.
- **Layout regen**: `make gen-layout` after any change to `bootstrap_layout.h`.
- **`go.mod` unchanged** — no new dependencies. `zig` is the only external toolchain, and only when actually rebuilding the payload.

## 7. Impact on non-Windows contributors — zero

| Scenario | Requires zig? | Requires CGO? | Default `go build ./...` succeeds? |
|---|---|---|---|
| macOS / Linux feature work | no | no | yes |
| Windows non-ABE (v10/DPAPI) | no | no | yes (stub path) |
| Windows release with ABE | **yes** | no | `make build-windows` |
| CI on any host (non-release) | no | no | yes |

All ABE-specific Go code is behind `//go:build windows` (plus `&& abe_embed` for the payload embed).

## 8. Zero disk footprint (enforced)

**No payload bytes ever touch disk on the target machine.**

- Payload DLL exists only as:
  1. Build artifact on the developer machine (`crypto/abe_extractor_amd64.bin`, git-ignored)
  2. `.rdata` section of `hack-browser-data.exe` (`//go:embed`)
  3. Go `[]byte` in our process memory (one `copy()` for import patching)
  4. `VirtualAllocEx`'d region in the target browser during injection; released on `TerminateProcess`

No `%TEMP%\*.dll` or `%TEMP%\*.txt`. The master key is handed back via `ReadProcessMemory` on the target's scratch region at `remoteBase + 0x40` (32 bytes). Everything stays in RAM.

### 8.1 Scratch layout

```
imageBase + 0x00  MZ header (untouched by us)
imageBase + 0x28  marker (1 B)              ← Bootstrap progress
imageBase + 0x29  key_status (1 B; 0x01 = ready)
imageBase + 0x2A  extract_err_code (1 B)    ← ABE_ERR_* category on failure
imageBase + 0x2C  hresult (4 B LE)          ← COM HRESULT on failure (0 on success)
imageBase + 0x30  com_err (4 B LE)          ← IElevator out DWORD on failure
imageBase + 0x3C  e_lfanew (PE header ptr, MUST NOT overwrite)
imageBase + 0x40..0x67  shared region (union):
                  pre-Bootstrap: 5 × uintptr (LoadLibraryA, GetProcAddress,
                                 VirtualAlloc, VirtualProtect, NtFlushIC)
                  post-DllMain : 32-byte master key at 0x40..0x5F
```

`0x40..0x5F` is **time-shared**: Go writes import pointers pre-injection; Bootstrap reads them once at function start; then DllMain overwrites the same bytes with the key. No concurrent readers.

## 9. Comparison with reference implementations

Three implementations of "extract Chrome v20 master key via reflective injection" exist in the ecosystem.

| Dimension | **This project** | **injector-old** (local C++ fork) | **xaitax/Chrome-App-Bound-Encryption-Decryption** |
|---|---|---|---|
| Top-level language | Go + C | Go + C++ | C++ end-to-end |
| Injector runtime | Go, `CGO_ENABLED=0` | Go, `CGO_ENABLED=0` | C++ standalone exe |
| Reflective loader | **Self-written C**, ~280 lines | Stephen Fewer 2012 `ReflectiveLoader` (vendored C, ~500) | Self-written C++, ~400 |
| kernel32 resolution | **Pre-resolved by Go, patched into DOS stub** | PEB walk + `_rotr` hash | PEB walk + `_rotr` hash |
| Syscall mechanism | Win32 APIs | Win32 APIs | Direct syscall via ASM trampoline |
| COM DecryptData dispatch | Vtable slot by browser kind (5/8/13) | Full interface via `ComPtr` | Same as injector-old |
| IPC payload → injector | **env var in, scratch-region read out** | Named pipe (full duplex) | Named pipe (full duplex) |
| Build toolchain for payload | `zig cc` | MSVC / clang-cl | MSVC |
| Runtime disk footprint | **0 bytes** | 1 temp file + pipe | Pipe |
| EDR evasion posture | None (Win32 APIs visible) | Partial (optional Nt*) | Strong (direct syscalls) |

### 9.1 Why we didn't vendor xaitax's Bootstrap

Tempting — it's known-good. But: C++ in an otherwise pure-C/Go repo; ASM trampolines + direct syscalls add a second toolchain leg; pipe-based IPC is 300+ lines of C we don't need; browser termination is a product-policy decision we skipped.

### 9.2 Why we abandoned Stephen Fewer's loader

`while(curr)` loop without `curr != head` termination → walked past end of the circular `InMemoryOrderModuleList` → dereferenced `PEB_LDR_DATA` itself as an `LDR_DATA_TABLE_ENTRY` → access-violated on `BaseDllName.pBuffer`. The 2012-era struct alignment hack (commented-out first `LIST_ENTRY`) also makes it brittle against Windows internals. Our replacement is strictly smaller, addresses these bugs explicitly, and is first-party.

## 10. Browser coverage

As of 2026-04-19, tested against Chrome 147 family.

| Browser class | Behavior | Status |
|---|---|---|
| Chrome Stable/Beta, Brave, CocCoc | ABE v20 via `CHROME_BASE` slot (5) | ✅ verified (cookies + passwords, zero non-ASCII in output) |
| Microsoft Edge | ABE v20 via `EDGE` slot (8); v2 `E_NOINTERFACE` → v1 fallback succeeds | ✅ verified |
| Avast Secure Browser | ABE v20 via `AVAST` slot (13) | ⚠️ table entry shipped; not yet sandbox-tested |
| Opera / OperaGX / Vivaldi / Yandex / Arc / 360 / QQ / Sogou | Not in `com_iid.c` | ⚠️ legacy v10 cookies still decrypt via DPAPI; v20 cookies do not |

Authoritative CLSID/IID table: `crypto/windows/abe_native/com_iid.c`.

## 11. Adding support for a new Chromium fork

Three steps. Detail (dump scripts, CLSID discovery) lives in private maintainer notes.

1. **Discover CLSID** — find the fork's elevation Windows service, look up its AppID in `HKLM\SOFTWARE\Classes\AppID`, then the CLSID that binds to it in `HKLM\SOFTWARE\Classes\CLSID`.
2. **Mine IIDs from TypeLib** — the interface IIDs live in the TypeLib resource of `<InstallDir>\Application\<version>\elevation_service.exe`. PowerShell + `ITypeLib.GetTypeInfo` enumerates them. Map `IElevator<Vendor>` → v1 IID, `IElevator2<Vendor>` → v2 IID (absent for older vendors).
3. **Determine vtable slot** — count `IElevator` methods in the TypeLib. Chrome-family has 3 methods (slot 5). Edge prepends 3 placeholders (slot 8). Avast extends the interface further (slot 13).

Edit `crypto/windows/abe_native/com_iid.c` (add the entry), `utils/winutil/browser_meta_windows.go` (add a matching `winutil.Entry` with the right `ABEKind` and install-path fallbacks), `browser/browser_windows.go` (set `Storage: "<key>"` for the new `BrowserConfig`), then `make payload-clean && make build-windows` and redeploy.

## 12. Known issues & future work

**Known**:

- Non-`com_iid.c` browsers (Opera, Vivaldi, Yandex, Arc, 360, QQ, Sogou) fall back to DPAPI; v20 cookies remain encrypted. Fix = §11 procedure per vendor.
- ARM64 Windows unsupported. Payload is `x86_64-windows-gnu` only. xaitax ships ARM64; we'd need parallel payload builds + runtime arch dispatch.
- Chrome v20 domain-binding prefix: injector-old strips 32 bytes at the start of v20 plaintext. Not observed on Chrome 147 sandbox outputs; left unimplemented. Re-add if a future test surfaces the prefix.
- Running-browser handling: if the user has the target browser open we spawn a second instance. No observed conflict, but some vendors (Opera GX) serialize elevation service; an opt-in `--kill-running` is future work.

**Future** (ordered by value):

1. Runtime CLSID/IID lookup from `elevation_service_idl.tlb` (no rebuild per fork rotation)
2. More forks via §11 (Opera, Vivaldi, Yandex, Arc)
3. x86 payload variant (for legacy 32-bit Chrome installs)
4. Optional `--kill-running` flag
5. EDR-hardened `injector.Strategy` variant (direct syscalls)
6. Release signing (cosign / SBOM) + reproducible-build CI verification
7. ARM64 Windows support

## 13. Related RFCs

| RFC | Relation |
|---|---|
| [RFC-003 Chromium Encryption](003-chromium-encryption.md) | v10/v11/v20 cipher format reference; v20 now implemented on Windows per this RFC |
| [RFC-006 Key Retrieval](006-key-retrieval-mechanisms.md) | `keyretriever.Retrievers` taxonomy; Windows populates V10 (DPAPI) + V20 (ABE) as independent tier slots |
| [RFC-009 Windows Locked Files](009-windows-locked-file-bypass.md) | Sibling Windows-specific workaround (handle duplication for locked DBs) |
