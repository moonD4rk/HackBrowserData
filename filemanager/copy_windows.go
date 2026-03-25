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

	data, err := readFileContent(handle)
	if err != nil {
		return fmt.Errorf("read via file mapping: %w", err)
	}

	return os.WriteFile(dst, data, 0o600)
}

// findFileHandle enumerates all system handles, finds the one matching the
// target file path, and duplicates it into the current process.
func findFileHandle(targetPath string) (windows.Handle, error) {
	// Extract a stable suffix for matching that avoids short path name issues
	// (e.g., RUNNER~1 vs runneradmin in the username portion).
	// We match from AppData onwards, which uniquely identifies each browser:
	//   Google\Chrome\User Data\Default\Network\Cookies  (Chrome)
	//   Microsoft\Edge\User Data\Default\Network\Cookies (Edge)
	targetSuffix := extractStableSuffix(targetPath)
	currentProcess := windows.CurrentProcess()

	handles, err := querySystemHandles()
	if err != nil {
		return 0, err
	}

	for _, h := range handles {
		pid := uint32(h.UniqueProcessID)
		if pid == 0 {
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

		if strings.HasSuffix(strings.ToLower(name), targetSuffix) {
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

// readFileContent reads file content from a duplicated handle.
// It uses FileMapping first (CreateFileMapping + MapViewOfFile), which reads
// from the OS kernel's file cache — this includes WAL data that Chrome has
// written but not yet checkpointed to the main file. Falls back to ReadFile
// if FileMapping fails.
func readFileContent(handle windows.Handle) ([]byte, error) {
	// Get file size
	var fileSize int64
	ret, _, sizeErr := procGetFileSizeEx.Call(
		uintptr(handle),
		uintptr(unsafe.Pointer(&fileSize)),
	)
	if ret == 0 {
		return nil, fmt.Errorf("GetFileSizeEx: %w", sizeErr)
	}
	if fileSize == 0 {
		return nil, fmt.Errorf("file is empty")
	}

	size := int(fileSize)

	// Try FileMapping first — reads from kernel file cache, includes WAL data
	if data, err := readViaFileMapping(handle, size); err == nil {
		return data, nil
	}

	// FileMapping failed, fall back to ReadFile
	// Seek to beginning first — the handle's file pointer may be at an arbitrary position
	if _, err := windows.Seek(handle, 0, 0); err != nil {
		return nil, fmt.Errorf("seek to start: %w", err)
	}
	data := make([]byte, size)
	var bytesRead uint32
	if err := windows.ReadFile(handle, data, &bytesRead, nil); err != nil {
		return nil, fmt.Errorf("ReadFile: %w", err)
	}
	return data[:bytesRead], nil
}

// readViaFileMapping reads file content using CreateFileMapping + MapViewOfFile.
func readViaFileMapping(handle windows.Handle, size int) ([]byte, error) {
	mapping, _, err := procCreateFileMappingW.Call(
		uintptr(handle),
		0, pageReadonly,
		0, 0, 0,
	)
	if mapping == 0 {
		return nil, fmt.Errorf("CreateFileMapping: %w", err)
	}
	defer windows.CloseHandle(windows.Handle(mapping))

	viewPtr, _, err := procMapViewOfFile.Call(
		mapping, fileMAPRead,
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

// extractStableSuffix extracts a path suffix that is stable across short/long
// path name variations. It finds "AppData" in the path and returns everything
// after "AppData\Local\" or "AppData\Roaming\" in lowercase.
//
// Example:
//
//	C:\Users\RUNNER~1\AppData\Local\Google\Chrome\...\Cookies
//	→ google\chrome\...\cookies
//
// For paths without "AppData" (e.g., test temp dirs), it falls back to
// the last 3 path components to provide reasonable matching specificity.
func extractStableSuffix(path string) string {
	lower := strings.ToLower(path)
	// Try to find AppData\Local\ or AppData\Roaming\
	for _, marker := range []string{`appdata\local\`, `appdata\roaming\`} {
		if idx := strings.Index(lower, marker); idx != -1 {
			return lower[idx+len(marker):]
		}
	}
	// Fallback: use last 3 components for test paths
	parts := strings.Split(lower, string(os.PathSeparator))
	if len(parts) >= 3 {
		return strings.Join(parts[len(parts)-3:], string(os.PathSeparator))
	}
	return lower
}
