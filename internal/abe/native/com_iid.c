#include "com_iid.h"

// CLSID / IID values migrated from HackBrowserData-injector-old's
// browser_config.hpp and cross-checked against each vendor's Chromium
// fork. Keep the per-entry comments with the GUID source so future
// rotations can be traced.
static const BrowserComIds kBrowsers[] = {
    // Chrome Stable
    // CLSID:  {708860E0-F641-4611-8895-7D867DD3675B}
    // v1 IID: {463ABECF-410D-407F-8AF5-0DF35A005CC8}  IElevatorChrome
    // v2 IID: {1BF5208B-295F-4992-B5F4-3A9BB6494838}  IElevator2Chrome
    {
        "chrome.exe", BROWSER_CHROME_BASE,
        { 0x708860E0, 0xF641, 0x4611, { 0x88, 0x95, 0x7D, 0x86, 0x7D, 0xD3, 0x67, 0x5B } },
        { 0x463ABECF, 0x410D, 0x407F, { 0x8A, 0xF5, 0x0D, 0xF3, 0x5A, 0x00, 0x5C, 0xC8 } },
        TRUE,
        { 0x1BF5208B, 0x295F, 0x4992, { 0xB5, 0xF4, 0x3A, 0x9B, 0xB6, 0x49, 0x48, 0x38 } },
    },

    // Chrome Beta — shares chrome.exe basename; the first table hit wins,
    // so this entry is effectively dead until registry-based channel
    // detection lands. Kept for reference.
    // CLSID:  {DD2646BA-3707-4BF8-B9A7-038691A68FC2}
    // v1 IID: {A2721D66-376E-4D2F-9F0F-9070E9A42B5F}
    // v2 IID: {B96A14B8-D0B0-44D8-BA68-2385B2A03254}
    {
        "chrome.exe", BROWSER_CHROME_BASE,
        { 0xDD2646BA, 0x3707, 0x4BF8, { 0xB9, 0xA7, 0x03, 0x86, 0x91, 0xA6, 0x8F, 0xC2 } },
        { 0xA2721D66, 0x376E, 0x4D2F, { 0x9F, 0x0F, 0x90, 0x70, 0xE9, 0xA4, 0x2B, 0x5F } },
        TRUE,
        { 0xB96A14B8, 0xD0B0, 0x44D8, { 0xBA, 0x68, 0x23, 0x85, 0xB2, 0xA0, 0x32, 0x54 } },
    },

    // Brave
    // CLSID:  {576B31AF-6369-4B6B-8560-E4B203A97A8B}
    // v1 IID: {F396861E-0C8E-4C71-8256-2FAE6D759CE9}
    // v2 IID: {1BF5208B-295F-4992-B5F4-3A9BB6494838}  (same as Chrome)
    {
        "brave.exe", BROWSER_CHROME_BASE,
        { 0x576B31AF, 0x6369, 0x4B6B, { 0x85, 0x60, 0xE4, 0xB2, 0x03, 0xA9, 0x7A, 0x8B } },
        { 0xF396861E, 0x0C8E, 0x4C71, { 0x82, 0x56, 0x2F, 0xAE, 0x6D, 0x75, 0x9C, 0xE9 } },
        TRUE,
        { 0x1BF5208B, 0x295F, 0x4992, { 0xB5, 0xF4, 0x3A, 0x9B, 0xB6, 0x49, 0x48, 0x38 } },
    },

    // Microsoft Edge
    // CLSID:  {1FCBE96C-1697-43AF-9140-2897C7C69767}
    // v1 IID: {C9C2B807-7731-4F34-81B7-44FF7779522B}  IEdgeElevatorFinal
    // v2 IID: {8F7B6792-784D-4047-845D-1782EFBEF205}  IEdgeElevator2Final
    {
        "msedge.exe", BROWSER_EDGE,
        { 0x1FCBE96C, 0x1697, 0x43AF, { 0x91, 0x40, 0x28, 0x97, 0xC7, 0xC6, 0x97, 0x67 } },
        { 0xC9C2B807, 0x7731, 0x4F34, { 0x81, 0xB7, 0x44, 0xFF, 0x77, 0x79, 0x52, 0x2B } },
        TRUE,
        { 0x8F7B6792, 0x784D, 0x4047, { 0x84, 0x5D, 0x17, 0x82, 0xEF, 0xBE, 0xF2, 0x05 } },
    },

    // CocCoc Browser
    // Service: CocCocElevationService
    // CLSID:  {77358251-489E-46F6-AAD6-1D41B89FEF01}
    // v1 IID: {0E9BCC98-8138-417A-83C3-4D4AAFED6316}  IElevatorCocCoc
    // v2 IID: {7E26AA1D-1A19-4538-9780-D0B6A1A693E5}  IElevator2CocCoc
    //         (extracted via LoadTypeLibEx on elevation_service.exe)
    {
        "browser.exe", BROWSER_CHROME_BASE,
        { 0x77358251, 0x489E, 0x46F6, { 0xAA, 0xD6, 0x1D, 0x41, 0xB8, 0x9F, 0xEF, 0x01 } },
        { 0x0E9BCC98, 0x8138, 0x417A, { 0x83, 0xC3, 0x4D, 0x4A, 0xAF, 0xED, 0x63, 0x16 } },
        TRUE,
        { 0x7E26AA1D, 0x1A19, 0x4538, { 0x97, 0x80, 0xD0, 0xB6, 0xA1, 0xA6, 0x93, 0xE5 } },
    },

    // Avast Secure Browser
    // CLSID:  {EAD34EE8-8D08-4CA1-ADA3-64754374D811}
    // IID:    {7737BB9F-BAC1-4C71-A696-7C82D7994B6F}  IAvastElevator
    {
        "avastbrowser.exe", BROWSER_AVAST,
        { 0xEAD34EE8, 0x8D08, 0x4CA1, { 0xAD, 0xA3, 0x64, 0x75, 0x43, 0x74, 0xD8, 0x11 } },
        { 0x7737BB9F, 0xBAC1, 0x4C71, { 0xA6, 0x96, 0x7C, 0x82, 0xD7, 0x99, 0x4B, 0x6F } },
        FALSE,
        { 0 },
    },

    { NULL, BROWSER_UNKNOWN, { 0 }, { 0 }, FALSE, { 0 } },
};

static char ascii_tolower(char c) {
    return (c >= 'A' && c <= 'Z') ? (char)(c - 'A' + 'a') : c;
}

static int iequal_ascii(const char *a, const char *b) {
    for (; *a && *b; ++a, ++b) {
        if (ascii_tolower(*a) != ascii_tolower(*b)) return 0;
    }
    return *a == *b;
}

const BrowserComIds *LookupBrowserByExe(const char *exe_basename) {
    if (!exe_basename) {
        return NULL;
    }
    for (const BrowserComIds *p = kBrowsers; p->exe_basename != NULL; ++p) {
        if (iequal_ascii(p->exe_basename, exe_basename)) {
            return p;
        }
    }
    return NULL;
}

unsigned int DecryptDataVtblIndex(BrowserKind kind) {
    switch (kind) {
    case BROWSER_CHROME_BASE:
        return 5;
    case BROWSER_EDGE:
        return 8;
    case BROWSER_AVAST:
        return 13;
    default:
        return 0;
    }
}
