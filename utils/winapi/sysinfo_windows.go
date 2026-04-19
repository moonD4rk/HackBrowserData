//go:build windows

package winapi

import (
	"fmt"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	// systemExtendedHandleInformation is the information class for
	// NtQuerySystemInformation that returns SYSTEM_HANDLE_INFORMATION_EX.
	// This is the 64-bit safe version (class 64) — UniqueProcessId is
	// ULONG_PTR instead of USHORT, avoiding PID truncation on 64-bit Windows.
	systemExtendedHandleInformation = 64

	statusInfoLengthMismatch = 0xC0000004

	fileMapRead  = 0x0004
	pageReadonly = 0x02

	// FileTypeDisk is the GetFileType return value for a normal disk file.
	FileTypeDisk uint32 = 0x0001

	maxHandleBufferSize = 256 * 1024 * 1024
	initialHandleBuffer = 4 * 1024 * 1024
)

var (
	procNtQuerySystemInformation  = Ntdll.NewProc("NtQuerySystemInformation")
	procGetFileType               = Kernel32.NewProc("GetFileType")
	procGetFinalPathNameByHandleW = Kernel32.NewProc("GetFinalPathNameByHandleW")
	procCreateFileMappingW        = Kernel32.NewProc("CreateFileMappingW")
	procMapViewOfFile             = Kernel32.NewProc("MapViewOfFile")
	procUnmapViewOfFile           = Kernel32.NewProc("UnmapViewOfFile")
	procGetFileSizeEx             = Kernel32.NewProc("GetFileSizeEx")
)

// SystemHandleEntry mirrors SYSTEM_HANDLE_TABLE_ENTRY_INFO_EX, the extended
// entry returned by SystemExtendedHandleInformation (class 64).
//
// Layout on 64-bit Windows (40 bytes):
//
//	PVOID      Object;
//	ULONG_PTR  UniqueProcessId;
//	ULONG_PTR  HandleValue;
//	ULONG      GrantedAccess;
//	USHORT     CreatorBackTraceIndex;
//	USHORT     ObjectTypeIndex;
//	ULONG      HandleAttributes;
//	ULONG      Reserved;
type SystemHandleEntry struct {
	Object                uintptr
	UniqueProcessID       uintptr
	HandleValue           uintptr
	GrantedAccess         uint32
	CreatorBackTraceIndex uint16
	ObjectTypeIndex       uint16
	HandleAttributes      uint32
	Reserved              uint32
}

// QuerySystemHandles enumerates all open handles system-wide via
// NtQuerySystemInformation(SystemExtendedHandleInformation). The buffer
// size grows on STATUS_INFO_LENGTH_MISMATCH until it succeeds or exceeds
// the safety cap.
func QuerySystemHandles() ([]SystemHandleEntry, error) {
	bufSize := uint32(initialHandleBuffer)

	for {
		buf := make([]byte, bufSize)
		var returnLength uint32

		ret, _, _ := procNtQuerySystemInformation.Call(
			systemExtendedHandleInformation,
			uintptr(unsafe.Pointer(&buf[0])),
			uintptr(bufSize),
			uintptr(unsafe.Pointer(&returnLength)),
		)

		if ret == statusInfoLengthMismatch {
			bufSize *= 2
			if bufSize > maxHandleBufferSize {
				return nil, fmt.Errorf("handle info buffer exceeded %d bytes", maxHandleBufferSize)
			}
			continue
		}
		if ret != 0 {
			return nil, fmt.Errorf("NtQuerySystemInformation returned 0x%x", ret)
		}

		// Header on 64-bit: NumberOfHandles (ULONG_PTR) + Reserved (ULONG_PTR) = 16 bytes.
		numberOfHandles := *(*uintptr)(unsafe.Pointer(&buf[0]))
		if numberOfHandles == 0 {
			return nil, nil
		}

		count := int(numberOfHandles)
		const headerSize = unsafe.Sizeof(uintptr(0)) * 2
		entrySize := unsafe.Sizeof(SystemHandleEntry{})

		required := headerSize + uintptr(count)*entrySize
		if required > uintptr(len(buf)) {
			return nil, fmt.Errorf("buffer too small: need %d, have %d", required, len(buf))
		}

		entries := make([]SystemHandleEntry, count)
		for i := 0; i < count; i++ {
			src := unsafe.Pointer(uintptr(unsafe.Pointer(&buf[0])) + headerSize + uintptr(i)*entrySize)
			entries[i] = *(*SystemHandleEntry)(src)
		}
		return entries, nil
	}
}

// GetFileType returns the Windows FileType for h (e.g., FileTypeDisk).
func GetFileType(h windows.Handle) uint32 {
	t, _, _ := procGetFileType.Call(uintptr(h))
	return uint32(t)
}

// GetFileSizeEx returns the size of the file referenced by h.
func GetFileSizeEx(h windows.Handle) (int64, error) {
	var sz int64
	r, _, err := procGetFileSizeEx.Call(uintptr(h), uintptr(unsafe.Pointer(&sz)))
	if r == 0 {
		return 0, fmt.Errorf("GetFileSizeEx: %w", err)
	}
	return sz, nil
}

// ExpandEnvString is the Go-friendly wrapper around
// kernel32!ExpandEnvironmentStringsW. Use it when you need to resolve
// Windows-style %VAR% placeholders — Go's stdlib os.ExpandEnv only
// recognizes Unix-style $VAR / ${VAR} and leaves %VAR% untouched.
func ExpandEnvString(s string) (string, error) {
	src, err := windows.UTF16PtrFromString(s)
	if err != nil {
		return "", fmt.Errorf("ExpandEnvString: %w", err)
	}
	// 4 KB of UTF-16 easily covers MAX_PATH-bounded install locations.
	buf := make([]uint16, 4096)
	n, err := windows.ExpandEnvironmentStrings(src, &buf[0], uint32(len(buf)))
	if n == 0 {
		return "", fmt.Errorf("ExpandEnvironmentStringsW: %w", err)
	}
	if int(n) > len(buf) {
		// Buffer was too small — retry with exact size.
		buf = make([]uint16, n)
		n, err = windows.ExpandEnvironmentStrings(src, &buf[0], uint32(len(buf)))
		if n == 0 {
			return "", fmt.Errorf("ExpandEnvironmentStringsW: %w", err)
		}
	}
	return windows.UTF16ToString(buf[:n]), nil
}

// GetFinalPathName returns the normalized file path for h, with the
// \\?\ prefix stripped.
func GetFinalPathName(h windows.Handle) (string, error) {
	size := 512
	for {
		buf := make([]uint16, size)
		n, _, err := procGetFinalPathNameByHandleW.Call(
			uintptr(h),
			uintptr(unsafe.Pointer(&buf[0])),
			uintptr(len(buf)),
			0, // FILE_NAME_NORMALIZED
		)
		if n == 0 {
			return "", fmt.Errorf("GetFinalPathNameByHandle: %w", err)
		}
		if int(n) > len(buf) {
			size = int(n)
			continue
		}
		path := windows.UTF16ToString(buf[:n])
		return strings.TrimPrefix(path, `\\?\`), nil
	}
}

// MapFile creates a read-only file mapping over h, copies the first
// size bytes into a Go-owned slice, and releases the mapping. Reads go
// through the OS kernel's file cache, which includes SQLite WAL data
// that has not yet been checkpointed into the main file.
func MapFile(h windows.Handle, size int) ([]byte, error) {
	mapping, _, err := procCreateFileMappingW.Call(
		uintptr(h),
		0, pageReadonly,
		0, 0, 0,
	)
	if mapping == 0 {
		return nil, fmt.Errorf("CreateFileMapping: %w", err)
	}
	defer windows.CloseHandle(windows.Handle(mapping))

	viewPtr, _, err := procMapViewOfFile.Call(
		mapping, fileMapRead,
		0, 0, 0,
	)
	if viewPtr == 0 {
		return nil, fmt.Errorf("MapViewOfFile: %w", err)
	}
	defer procUnmapViewOfFile.Call(viewPtr)

	// viewPtr is a valid pointer from MapViewOfFile syscall.
	// go vet flags this as "possible misuse of unsafe.Pointer" but it's
	// correct usage for Windows memory-mapped I/O.
	data := make([]byte, size)
	copy(data, (*[1 << 30]byte)(unsafe.Pointer(viewPtr))[:size]) //nolint:govet
	return data, nil
}
