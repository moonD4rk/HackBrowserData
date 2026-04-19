//go:build windows

package injector

import (
	"errors"
	"fmt"
	"syscall"

	"golang.org/x/sys/windows"
)

// Package-level lazy DLL/Proc handles. Consolidating them here avoids the
// NewLazySystemDLL("kernel32.dll") boilerplate spread across every helper in
// reflective_windows.go, and gives us a single place to extend when a new
// Win32 API is needed. Matches the pattern used in filemanager/copy_windows.go.
var (
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")
	ntdll    = windows.NewLazySystemDLL("ntdll.dll")

	// Call-style procs: Win32 APIs that `golang.org/x/sys/windows` does NOT
	// provide typed wrappers for. We invoke them via LazyProc.Call.
	procVirtualAllocEx     = kernel32.NewProc("VirtualAllocEx")
	procCreateRemoteThread = kernel32.NewProc("CreateRemoteThread")

	// Address-style procs: consumed only via .Addr() by patchPreresolvedImports
	// to patch raw function pointers into the payload's DOS stub. We never Call
	// these from our own process.
	procLoadLibraryA   = kernel32.NewProc("LoadLibraryA")
	procGetProcAddress = kernel32.NewProc("GetProcAddress")
	procVirtualAlloc   = kernel32.NewProc("VirtualAlloc")
	procVirtualProtect = kernel32.NewProc("VirtualProtect")
	procNtFlushIC      = ntdll.NewProc("NtFlushInstructionCache")
)

// callBoolErr wraps the common "r1 == 0 means failure" Win32 convention.
// Win32 GetLastError often returns ERROR_SUCCESS (errno 0) even on failure,
// so we distinguish the "no-errno" case explicitly to avoid emitting misleading
// "operation completed successfully" messages. We use errors.As rather than a
// type assertion so the check stays correct if x/sys/windows ever wraps the
// underlying errno.
func callBoolErr(p *windows.LazyProc, args ...uintptr) (uintptr, error) {
	r, _, callErr := p.Call(args...)
	if r == 0 {
		var errno syscall.Errno
		if errors.As(callErr, &errno) && errno == 0 {
			return 0, fmt.Errorf("%s: failed (no errno)", p.Name)
		}
		return 0, fmt.Errorf("%s: %w", p.Name, callErr)
	}
	return r, nil
}
