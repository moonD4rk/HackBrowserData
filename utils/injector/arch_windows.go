//go:build windows

package injector

import (
	"bytes"
	"debug/pe"
	"fmt"
)

type Arch string

const (
	ArchAMD64   Arch = "amd64"
	Arch386     Arch = "386"
	ArchUnknown Arch = "unknown"
)

func DetectPEArch(peBytes []byte) (Arch, error) {
	f, err := pe.NewFile(bytes.NewReader(peBytes))
	if err != nil {
		return ArchUnknown, fmt.Errorf("parse PE: %w", err)
	}
	switch f.Machine {
	case pe.IMAGE_FILE_MACHINE_AMD64:
		return ArchAMD64, nil
	case pe.IMAGE_FILE_MACHINE_I386:
		return Arch386, nil
	default:
		return ArchUnknown, nil
	}
}
