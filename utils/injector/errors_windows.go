//go:build windows

package injector

import (
	"fmt"

	"github.com/moond4rk/hackbrowserdata/crypto/windows/abe_native/bootstrap"
)

// abeErrNames maps the payload's ABE_ERR_* category byte (written into
// the scratch region at extract_err_code) to a human-readable cause.
var abeErrNames = map[byte]string{
	bootstrap.ErrBasename:       "basename extraction failed",
	bootstrap.ErrBrowserUnknown: "browser not in com_iid table",
	bootstrap.ErrEnvMissing:     "HBD_ABE_ENC_B64 env var missing or oversized",
	bootstrap.ErrBase64:         "base64 decode failed",
	bootstrap.ErrBstrAlloc:      "SysAllocStringByteLen failed",
	bootstrap.ErrComCreate:      "CoCreateInstance failed (no usable IElevator)",
	bootstrap.ErrDecryptData:    "IElevator.DecryptData failed",
	bootstrap.ErrKeyLen:         "key length mismatch (want 32)",
}

// hresultNames covers HRESULT values we've actually observed or expect to
// observe on failure paths. Unknown values fall back to hex.
var hresultNames = map[uint32]string{
	0x80004002: "E_NOINTERFACE",
	0x80010108: "RPC_E_DISCONNECTED",
	0x80040154: "REGDB_E_CLASSNOTREG",
	0x80070005: "E_ACCESSDENIED",
	0x800706BA: "RPC_S_SERVER_UNAVAILABLE",
}

// formatABEError renders a scratchResult into a diagnostic string used when
// the payload did not publish a key. The format is stable for greppability:
//
//	err=<category>, hr=<name> (0xXXXXXXXX), comErr=0xXXXXXXXX, marker=0xXX
func formatABEError(r scratchResult) string {
	errName := fmt.Sprintf("0x%02x", r.ErrCode)
	if n, ok := abeErrNames[r.ErrCode]; ok {
		errName = n
	}
	hrName := fmt.Sprintf("0x%08x", r.HResult)
	if n, ok := hresultNames[r.HResult]; ok {
		hrName = fmt.Sprintf("%s (0x%08x)", n, r.HResult)
	}
	return fmt.Sprintf("err=%s, hr=%s, comErr=0x%x, marker=0x%02x",
		errName, hrName, r.ComErr, r.Marker)
}
