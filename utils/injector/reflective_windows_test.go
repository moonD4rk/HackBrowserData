//go:build windows && amd64

package injector

import (
	"encoding/binary"
	"testing"

	"github.com/moond4rk/hackbrowserdata/crypto/windows/abe_native/bootstrap"
	"github.com/moond4rk/hackbrowserdata/utils/winapi"
)

// The C payload reads BootstrapParams by field offset while Go writes it as a flat byte
// slice, so this pins the encoding to the cgo-generated offsets rather than to field order.
func TestEncodeBootstrapParams(t *testing.T) {
	const scratchBase = 0xDEADBEEF000

	params, err := encodeBootstrapParams(scratchBase)
	if err != nil {
		t.Fatalf("encodeBootstrapParams: %v", err)
	}
	if len(params) != bootstrap.ParamsSize {
		t.Fatalf("params size = %d, want %d (C sizeof(BootstrapParams))", len(params), bootstrap.ParamsSize)
	}

	cases := []struct {
		name   string
		offset int
		want   uintptr
	}{
		{"scratch_base", bootstrap.ParamScratchBase, scratchBase},
		{"LoadLibraryA", bootstrap.ParamLoadLibraryA, winapi.AddrLoadLibraryA()},
		{"GetProcAddress", bootstrap.ParamGetProcAddress, winapi.AddrGetProcAddress()},
		{"VirtualAlloc", bootstrap.ParamVirtualAlloc, winapi.AddrVirtualAlloc()},
		{"VirtualProtect", bootstrap.ParamVirtualProtect, winapi.AddrVirtualProtect()},
		{"NtFlushInstructionCache", bootstrap.ParamNtFlushIC, winapi.AddrNtFlushInstructionCache()},
	}
	for _, tc := range cases {
		got := binary.LittleEndian.Uint64(params[tc.offset : tc.offset+8])
		if got != uint64(tc.want) {
			t.Errorf("%s at offset 0x%x = %#x, want %#x", tc.name, tc.offset, got, tc.want)
		}
		if tc.want == 0 {
			t.Errorf("%s resolved to a null address", tc.name)
		}
	}
}

func TestPutBootstrapParamBounds(t *testing.T) {
	const want = uintptr(0x0123456789ABCDEF)
	tests := []struct {
		name    string
		size    int
		offset  int
		wantErr bool
	}{
		{name: "last valid field", size: 16, offset: 8},
		{name: "negative offset", size: 8, offset: -1, wantErr: true},
		{name: "short block", size: 7, offset: 0, wantErr: true},
		{name: "one past end", size: 8, offset: 1, wantErr: true},
		{name: "maximum offset", size: 8, offset: int(^uint(0) >> 1), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := make([]byte, tt.size)
			err := putBootstrapParam(params, tt.offset, want)
			if tt.wantErr {
				if err == nil {
					t.Fatal("putBootstrapParam returned nil error")
				}
				return
			}
			if err != nil {
				t.Fatalf("putBootstrapParam: %v", err)
			}
			if got := binary.LittleEndian.Uint64(params[tt.offset:]); got != uint64(want) {
				t.Errorf("encoded value = %#x, want %#x", got, want)
			}
		})
	}
}

// Every field must land inside the struct and none may overlap — a stale offset constant
// would otherwise corrupt a neighboring pointer instead of failing loudly.
func TestBootstrapParamOffsetsAreDisjoint(t *testing.T) {
	offsets := []int{
		bootstrap.ParamScratchBase,
		bootstrap.ParamLoadLibraryA,
		bootstrap.ParamGetProcAddress,
		bootstrap.ParamVirtualAlloc,
		bootstrap.ParamVirtualProtect,
		bootstrap.ParamNtFlushIC,
	}
	seen := make(map[int]bool, len(offsets))
	for _, off := range offsets {
		if off%8 != 0 {
			t.Errorf("offset 0x%x is not 8-byte aligned", off)
		}
		if off < 0 || off > bootstrap.ParamsSize-bootstrapParamFieldSize {
			t.Errorf("offset 0x%x overruns params block of %d bytes", off, bootstrap.ParamsSize)
		}
		if seen[off] {
			t.Errorf("offset 0x%x used by more than one field", off)
		}
		seen[off] = true
	}
	if len(seen)*bootstrapParamFieldSize != bootstrap.ParamsSize {
		t.Errorf("%d fields cover %d bytes, but C sizeof(BootstrapParams) is %d",
			len(seen), len(seen)*bootstrapParamFieldSize, bootstrap.ParamsSize)
	}
}
