//go:build windows && !abe_embed

package crypto

import "fmt"

func getPayloadForArch(arch string) ([]byte, error) {
	return nil, fmt.Errorf(
		"abe: payload not embedded in this build (rebuild with -tags abe_embed; arch=%s)",
		arch,
	)
}
