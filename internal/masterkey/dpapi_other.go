//go:build !windows

package masterkey

import (
	"fmt"
	"runtime"
)

func DecryptDPAPI(_ []byte) ([]byte, error) {
	return nil, fmt.Errorf("DPAPI on %s: %w", runtime.GOOS, ErrNotApplicable)
}
