//go:build windows

package injector

import (
	"strings"
	"testing"

	"github.com/moond4rk/hackbrowserdata/crypto/windows/abe_native/bootstrap"
)

func TestFormatABEError(t *testing.T) {
	cases := []struct {
		name   string
		result scratchResult
		wants  []string
	}{
		{
			name: "known err code with known HRESULT",
			result: scratchResult{
				Marker:  0xff,
				Status:  0x00,
				ErrCode: bootstrap.ErrDecryptData,
				HResult: 0x80070005,
				ComErr:  0,
			},
			wants: []string{
				"err=IElevator.DecryptData failed",
				"hr=E_ACCESSDENIED (0x80070005)",
				"comErr=0x0",
				"marker=0xff",
			},
		},
		{
			name: "known err code, unknown HRESULT falls back to hex",
			result: scratchResult{
				Marker:  0xff,
				Status:  0x00,
				ErrCode: bootstrap.ErrBrowserUnknown,
				HResult: 0x12345678,
			},
			wants: []string{
				"err=browser not in com_iid table",
				"hr=0x12345678",
			},
		},
		{
			name: "unknown err code falls back to hex",
			result: scratchResult{
				ErrCode: 0xaa,
				HResult: 0,
			},
			wants: []string{
				"err=0xaa",
				"hr=0x00000000",
			},
		},
		{
			name: "err code zero (ok) also renders",
			result: scratchResult{
				ErrCode: bootstrap.ErrOk,
			},
			wants: []string{
				"err=0x00",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := formatABEError(tc.result)
			for _, want := range tc.wants {
				if !strings.Contains(got, want) {
					t.Errorf("formatABEError missing %q\n  got: %s", want, got)
				}
			}
		})
	}
}
