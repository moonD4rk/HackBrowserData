//go:build windows && abe_embed

// Package payload holds the compiled reflective-injection ABE payload
// binary and exposes it to the rest of HackBrowserData. The `abe_embed`
// build tag selects between a real //go:embed'd binary (this file) and
// a stub (stub_windows.go) so the default `go build ./...` succeeds on
// machines without the zig toolchain.
package payload

import (
	_ "embed"
	"fmt"
)

//go:generate make -C ../../.. payload

//go:embed abe_extractor_amd64.bin
var abePayloadAmd64 []byte

// Get returns the embedded ABE payload for the given architecture.
// Only "amd64" is supported today; x86 / ARM64 payloads are future work.
func Get(arch string) ([]byte, error) {
	switch arch {
	case "amd64":
		if len(abePayloadAmd64) == 0 {
			return nil, fmt.Errorf("abe: amd64 payload is empty (build system bug)")
		}
		return abePayloadAmd64, nil
	default:
		return nil, fmt.Errorf("abe: arch %q not supported in this build", arch)
	}
}
