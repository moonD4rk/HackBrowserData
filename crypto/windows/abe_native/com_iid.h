#ifndef HBD_ABE_COM_IID_H
#define HBD_ABE_COM_IID_H

#define WIN32_LEAN_AND_MEAN
#include <windows.h>

typedef enum BrowserKind {
    BROWSER_UNKNOWN = 0,
    BROWSER_CHROME_BASE,   // DecryptData at vtable slot 5
    BROWSER_EDGE,          // DecryptData at vtable slot 8
    BROWSER_AVAST,         // DecryptData at vtable slot 13
} BrowserKind;

typedef struct BrowserComIds {
    const char *exe_basename;
    BrowserKind kind;
    GUID clsid;
    GUID iid_v1;
    BOOL has_iid_v2;
    GUID iid_v2;
} BrowserComIds;

const BrowserComIds *LookupBrowserByExe(const char *exe_basename);

unsigned int DecryptDataVtblIndex(BrowserKind kind);

#endif // HBD_ABE_COM_IID_H
