//go:build windows

package injector

import (
	"os"
	"testing"
)

// kernel32Path is always present on a Windows host and its export table
// is stable across versions — an ideal fixture for PE-parsing tests.
const kernel32Path = `C:\Windows\System32\kernel32.dll`

func readKernel32(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(kernel32Path)
	if err != nil {
		t.Fatalf("read %s: %v", kernel32Path, err)
	}
	if len(data) == 0 {
		t.Fatalf("%s is empty", kernel32Path)
	}
	return data
}

func TestDetectPEArch_Kernel32IsAMD64(t *testing.T) {
	arch, err := DetectPEArch(readKernel32(t))
	if err != nil {
		t.Fatalf("DetectPEArch: %v", err)
	}
	if arch != ArchAMD64 {
		t.Errorf("expected %q, got %q", ArchAMD64, arch)
	}
}

func TestDetectPEArch_Garbage(t *testing.T) {
	_, err := DetectPEArch([]byte("definitely not a PE file"))
	if err == nil {
		t.Error("expected parse error for non-PE input")
	}
}

func TestDetectPEArch_EmptyInput(t *testing.T) {
	_, err := DetectPEArch(nil)
	if err == nil {
		t.Error("expected parse error for empty input")
	}
}

// TestFindExportFileOffset_KnownExports walks a handful of kernel32 exports
// that Bootstrap also relies on via pre-resolved import patching. Passing
// here means both the export-table walker and the RVA→file-offset converter
// work against a real Windows PE — not just against fixtures we control.
func TestFindExportFileOffset_KnownExports(t *testing.T) {
	data := readKernel32(t)

	// All of these have been stable kernel32 exports since Windows XP.
	exports := []string{
		"LoadLibraryA",
		"GetProcAddress",
		"VirtualAlloc",
		"VirtualProtect",
		"CreateFileW",
	}

	for _, name := range exports {
		off, err := FindExportFileOffset(data, name)
		if err != nil {
			t.Errorf("%s: unexpected error: %v", name, err)
			continue
		}
		if off == 0 {
			t.Errorf("%s: got zero file offset", name)
		}
		if int(off) >= len(data) {
			t.Errorf("%s: offset %d exceeds file size %d", name, off, len(data))
		}
	}
}

func TestFindExportFileOffset_MissingExport(t *testing.T) {
	data := readKernel32(t)
	_, err := FindExportFileOffset(data, "HbdNoSuchExport_abcdef1234")
	if err == nil {
		t.Error("expected error for nonexistent export name")
	}
}

func TestFindExportFileOffset_GarbageInput(t *testing.T) {
	_, err := FindExportFileOffset([]byte("not a PE file at all"), "LoadLibraryA")
	if err == nil {
		t.Error("expected error when parsing non-PE input")
	}
}

func TestFindExportFileOffset_TruncatedPE(t *testing.T) {
	data := readKernel32(t)
	// Chop to just the DOS stub — export directory is unreachable.
	if len(data) < 128 {
		t.Fatalf("kernel32 is implausibly small: %d bytes", len(data))
	}
	_, err := FindExportFileOffset(data[:128], "LoadLibraryA")
	if err == nil {
		t.Error("expected error when PE is truncated past the DOS header")
	}
}
