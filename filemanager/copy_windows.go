//go:build windows

package filemanager

import (
	"fmt"
	"os"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	// systemExtendedHandleInformation is the information class for
	// NtQuerySystemInformation that returns SYSTEM_HANDLE_INFORMATION_EX.
	// This is the 64-bit safe version (class 64) — UniqueProcessId is ULONG_PTR
	// instead of USHORT, avoiding PID truncation on 64-bit Windows.
	systemExtendedHandleInformation = 64

	statusInfoLengthMismatch = 0xC0000004

	fileMAPRead  = 0x0004
	pageReadonly = 0x02
	fileTypeDisk = 0x0001
)

// systemHandleTableEntryInfoEx represents SYSTEM_HANDLE_TABLE_ENTRY_INFO_EX.
// This is the extended version returned by SystemExtendedHandleInformation (class 64).
//
// Layout (64-bit Windows):
//
//	PVOID      Object;               // 8 bytes
//	ULONG_PTR  UniqueProcessId;      // 8 bytes
//	ULONG_PTR  HandleValue;          // 8 bytes
//	ULONG      GrantedAccess;        // 4 bytes
//	USHORT     CreatorBackTraceIndex; // 2 bytes
//	USHORT     ObjectTypeIndex;      // 2 bytes
//	ULONG      HandleAttributes;     // 4 bytes
//	ULONG      Reserved;             // 4 bytes
//	                          Total: 40 bytes on 64-bit
type systemHandleTableEntryInfoEx struct {
	Object                uintptr
	UniqueProcessID       uintptr // ULONG_PTR: safe for PID > 65535
	HandleValue           uintptr // ULONG_PTR: safe for large handle values
	GrantedAccess         uint32
	CreatorBackTraceIndex uint16
	ObjectTypeIndex       uint16
	HandleAttributes      uint32
	Reserved              uint32
}

var (
	ntdll                        = windows.NewLazySystemDLL("ntdll.dll")
	procNtQuerySystemInformation = ntdll.NewProc("NtQuerySystemInformation")

	kernel32                      = windows.NewLazySystemDLL("kernel32.dll")
	procGetFileType               = kernel32.NewProc("GetFileType")
	procGetFinalPathNameByHandleW = kernel32.NewProc("GetFinalPathNameByHandleW")
	procCreateFileMappingW        = kernel32.NewProc("CreateFileMappingW")
	procMapViewOfFile             = kernel32.NewProc("MapViewOfFile")
	procUnmapViewOfFile           = kernel32.NewProc("UnmapViewOfFile")
	procGetFileSizeEx             = kernel32.NewProc("GetFileSizeEx")
)

// copyLocked copies a file that is locked by another process (e.g., Chrome's
// Cookies database with PRAGMA locking_mode=EXCLUSIVE).
//
// Approach: DuplicateHandle + FileMapping
//  1. Enumerate all open file handles via NtQuerySystemInformation(SystemExtendedHandleInformation)
//  2. Find the handle matching the target file path
//  3. Duplicate that handle into our process via DuplicateHandle
//  4. Read file content through memory-mapped I/O (CreateFileMapping + MapViewOfFile)
//  5. Write content to destination
//
// This requires only normal user privileges (no admin needed).
func copyLocked(src, dst string) error {
	handle, err := findFileHandle(src)
	if err != nil {
		return fmt.Errorf("find file handle for %s: %w", src, err)
	}
	defer windows.CloseHandle(handle)

	data, err := readViaFileMapping(handle)
	if err != nil {
		return fmt.Errorf("read via file mapping: %w", err)
	}

	return os.WriteFile(dst, data, 0o600)
}

// findFileHandle enumerates all system handles, finds the one matching the
// target file path, and duplicates it into the current process.
func findFileHandle(targetPath string) (windows.Handle, error) {
	// Build match suffix: e.g., "Network\Cookies" from full path
	targetSuffix := buildMatchSuffix(targetPath)
	currentProcess := windows.CurrentProcess()

	handles, err := querySystemHandles()
	if err != nil {
		return 0, err
	}

	for _, h := range handles {
		pid := uint32(h.UniqueProcessID)
		if pid == 0 || pid == uint32(os.Getpid()) {
			continue
		}

		// Open the owning process to duplicate its handle
		process, err := windows.OpenProcess(windows.PROCESS_DUP_HANDLE, false, pid)
		if err != nil {
			continue
		}

		// Duplicate the handle into our process
		var dupHandle windows.Handle
		err = windows.DuplicateHandle(
			process,
			windows.Handle(h.HandleValue),
			currentProcess,
			&dupHandle,
			0, false,
			windows.DUPLICATE_SAME_ACCESS,
		)
		windows.CloseHandle(process)
		if err != nil {
			continue
		}

		// Verify it's a disk file (not a pipe, device, etc.)
		fileType, _, _ := procGetFileType.Call(uintptr(dupHandle))
		if fileType != fileTypeDisk {
			windows.CloseHandle(dupHandle)
			continue
		}

		// Get the file path and check if it matches our target
		name, err := getFinalPathName(dupHandle)
		if err != nil {
			windows.CloseHandle(dupHandle)
			continue
		}

		if matchPath(name, targetSuffix) {
			return dupHandle, nil
		}
		windows.CloseHandle(dupHandle)
	}

	return 0, fmt.Errorf("no process has file open: %s", targetPath)
}

// querySystemHandles calls NtQuerySystemInformation with
// SystemExtendedHandleInformation (class 64) to enumerate all open handles.
func querySystemHandles() ([]systemHandleTableEntryInfoEx, error) {
	bufSize := uint32(4 * 1024 * 1024) // start at 4 MB

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
			if bufSize > 256*1024*1024 {
				return nil, fmt.Errorf("handle info buffer exceeded 256 MB")
			}
			continue
		}
		if ret != 0 {
			return nil, fmt.Errorf("NtQuerySystemInformation returned 0x%x", ret)
		}

		// Parse: first field is NumberOfHandles (ULONG_PTR), then array of entries
		// On 64-bit: ULONG_PTR = 8 bytes
		numberOfHandles := *(*uintptr)(unsafe.Pointer(&buf[0]))
		if numberOfHandles == 0 {
			return nil, nil
		}

		count := int(numberOfHandles)
		// Entries start after NumberOfHandles + Reserved (both ULONG_PTR = 16 bytes total)
		const headerSize = unsafe.Sizeof(uintptr(0)) * 2
		entrySize := unsafe.Sizeof(systemHandleTableEntryInfoEx{})

		// Validate buffer bounds
		required := headerSize + uintptr(count)*entrySize
		if required > uintptr(len(buf)) {
			return nil, fmt.Errorf("buffer too small: need %d, have %d", required, len(buf))
		}

		entries := make([]systemHandleTableEntryInfoEx, count)
		for i := 0; i < count; i++ {
			src := unsafe.Pointer(uintptr(unsafe.Pointer(&buf[0])) + headerSize + uintptr(i)*entrySize)
			entries[i] = *(*systemHandleTableEntryInfoEx)(src)
		}
		return entries, nil
	}
}

// getFinalPathName returns the normalized file path for a file handle.
func getFinalPathName(handle windows.Handle) (string, error) {
	size := 512
	for {
		buf := make([]uint16, size)
		n, _, err := procGetFinalPathNameByHandleW.Call(
			uintptr(handle),
			uintptr(unsafe.Pointer(&buf[0])),
			uintptr(len(buf)),
			0, // FILE_NAME_NORMALIZED
		)
		if n == 0 {
			return "", fmt.Errorf("GetFinalPathNameByHandle: %w", err)
		}
		if int(n) > len(buf) {
			// Buffer too small, retry with required size
			size = int(n)
			continue
		}

		path := windows.UTF16ToString(buf[:n])
		// Remove \\?\ prefix added by GetFinalPathNameByHandle
		path = strings.TrimPrefix(path, `\\?\`)
		return path, nil
	}
}

// readViaFileMapping reads file content using memory-mapped I/O.
// This works even when ReadFile fails on certain Chrome versions,
// because MapViewOfFile accesses the file through the memory manager
// rather than the file system I/O path.
func readViaFileMapping(handle windows.Handle) ([]byte, error) {
	// Get file size
	var fileSize int64
	ret, _, err := procGetFileSizeEx.Call(
		uintptr(handle),
		uintptr(unsafe.Pointer(&fileSize)),
	)
	if ret == 0 {
		return nil, fmt.Errorf("GetFileSizeEx: %w", err)
	}
	if fileSize == 0 {
		return nil, fmt.Errorf("file is empty")
	}

	// Create read-only file mapping
	mapping, _, err := procCreateFileMappingW.Call(
		uintptr(handle),
		0, pageReadonly,
		0, 0, // use file size
		0, // unnamed
	)
	if mapping == 0 {
		return nil, fmt.Errorf("CreateFileMapping: %w", err)
	}
	defer windows.CloseHandle(windows.Handle(mapping))

	// Map entire file into our address space
	viewPtr, _, err := procMapViewOfFile.Call(
		mapping, fileMAPRead,
		0, 0, 0, // offset=0, size=0 means entire file
	)
	if viewPtr == 0 {
		return nil, fmt.Errorf("MapViewOfFile: %w", err)
	}
	defer procUnmapViewOfFile.Call(viewPtr)

	// Copy mapped memory into a Go byte slice
	size := int(fileSize)
	data := make([]byte, size)
	copy(data, unsafe.Slice((*byte)(unsafe.Pointer(viewPtr)), size))
	return data, nil
}

// buildMatchSuffix extracts the last two path components for matching.
// e.g., "C:\Users\x\...\Network\Cookies" → "Network\Cookies"
func buildMatchSuffix(fullPath string) string {
	parts := strings.Split(fullPath, string(os.PathSeparator))
	if len(parts) >= 2 {
		return parts[len(parts)-2] + string(os.PathSeparator) + parts[len(parts)-1]
	}
	if len(parts) == 1 {
		return parts[0]
	}
	return fullPath
}

// matchPath checks if a file path ends with the expected suffix (case-insensitive).
func matchPath(path, suffix string) bool {
	return strings.HasSuffix(strings.ToLower(path), strings.ToLower(suffix))
}
