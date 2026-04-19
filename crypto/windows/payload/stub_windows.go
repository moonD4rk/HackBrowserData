//go:build windows && !abe_embed

package payload

import "fmt"

// Get returns an error in non-release builds so feature code that needs
// the payload fails fast with a clear message. Release builds (built
// with -tags abe_embed) replace this with the //go:embed'd binary.
func Get(arch string) ([]byte, error) {
	return nil, fmt.Errorf(
		"abe: payload not embedded in this build (rebuild with -tags abe_embed; arch=%s)",
		arch,
	)
}
