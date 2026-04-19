//go:build windows

package filemanager

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/sys/windows"

	"github.com/moond4rk/hackbrowserdata/utils/winapi"
)

// copyLocked copies a file that is locked by another process (e.g., Chrome's
// Cookies database with PRAGMA locking_mode=EXCLUSIVE).
//
// Approach: DuplicateHandle + FileMapping
//  1. Enumerate all open file handles via NtQuerySystemInformation
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

	handles, err := winapi.QuerySystemHandles()
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
		_ = windows.CloseHandle(process)
		if err != nil {
			continue
		}

		// Verify it's a disk file (not a pipe, device, etc.)
		if winapi.GetFileType(dupHandle) != winapi.FileTypeDisk {
			_ = windows.CloseHandle(dupHandle)
			continue
		}

		// Get the file path and check if it matches our target
		name, err := winapi.GetFinalPathName(dupHandle)
		if err != nil {
			_ = windows.CloseHandle(dupHandle)
			continue
		}

		if strings.HasSuffix(strings.ToLower(name), targetSuffix) {
			return dupHandle, nil
		}
		_ = windows.CloseHandle(dupHandle)
	}

	return 0, fmt.Errorf("no process has file open: %s", targetPath)
}

// readFileContent reads file content from a duplicated handle.
// It uses FileMapping first (CreateFileMapping + MapViewOfFile), which reads
// from the OS kernel's file cache — this includes WAL data that Chrome has
// written but not yet checkpointed to the main file. Falls back to ReadFile
// if FileMapping fails.
func readFileContent(handle windows.Handle) ([]byte, error) {
	fileSize, err := winapi.GetFileSizeEx(handle)
	if err != nil {
		return nil, err
	}
	if fileSize == 0 {
		return nil, fmt.Errorf("file is empty")
	}

	size := int(fileSize)

	// Try FileMapping first — reads from kernel file cache, includes WAL data
	if data, err := winapi.MapFile(handle, size); err == nil {
		return data, nil
	}

	// FileMapping failed, fall back to ReadFile.
	// Seek to beginning first — the handle's file pointer may be at an
	// arbitrary position.
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
