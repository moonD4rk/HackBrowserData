//go:build windows && abe_embed

package crypto

import (
	_ "embed"
	"fmt"
)

//go:generate make -C ../.. payload

//go:embed abe_extractor_amd64.bin
var abePayloadAmd64 []byte

func getPayloadForArch(arch string) ([]byte, error) {
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
