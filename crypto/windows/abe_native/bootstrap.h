// SPDX-License-Identifier: Apache-2.0

#ifndef HBD_ABE_BOOTSTRAP_H
#define HBD_ABE_BOOTSTRAP_H

#define WIN32_LEAN_AND_MEAN
#include <windows.h>

// Scratch layout inside imageBase (the raw-file payload region in the
// target process). Shared contract with the Go injector — keep the
// offsets and meanings in sync with utils/injector/reflective_windows.go.
//
//   0x28            step marker (written by Bootstrap)
//   0x29            key status   (0x01 = ready, written by DllMain)
//   0x40..0x67      pre-Bootstrap: 5 pre-resolved Win32 fn pointers
//                   post-DllMain : 32-byte master key at 0x40..0x5F
//
// Windows' PE loader ignores the DOS stub region (0x40..0x77), and
// Bootstrap only reads the imports once at function start, so DllMain
// can safely overwrite 0x40..0x5F with the key afterwards.

#define BOOTSTRAP_MARKER_OFFSET      0x28

#define BOOTSTRAP_KEY_STATUS_OFFSET  0x29
#define BOOTSTRAP_KEY_STATUS_READY   0x01

#define BOOTSTRAP_KEY_OFFSET         0x40
#define BOOTSTRAP_KEY_LEN            32

#define BOOTSTRAP_IMPORT_LOADLIBRARYA_OFFSET   0x40
#define BOOTSTRAP_IMPORT_GETPROCADDRESS_OFFSET 0x48
#define BOOTSTRAP_IMPORT_VIRTUALALLOC_OFFSET   0x50
#define BOOTSTRAP_IMPORT_VIRTUALPROTECT_OFFSET 0x58
#define BOOTSTRAP_IMPORT_NTFLUSHIC_OFFSET      0x60

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

#ifdef __cplusplus
extern "C" {
#endif

__declspec(dllexport) ULONG_PTR WINAPI Bootstrap(LPVOID lpParameter);

#ifdef __cplusplus
}
#endif

#endif // HBD_ABE_BOOTSTRAP_H
