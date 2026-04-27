//go:build windows

// Package winapi centralizes low-level Windows API access used across
// HackBrowserData. It exposes typed wrappers around specific syscalls
// that golang.org/x/sys/windows does not cover, plus shared LazyDLL
// handles and a small error-handling helper.
//
// Callers: utils/injector, filemanager, crypto. Higher-level Windows
// browser utilities live in utils/winutil.
package winapi

import (
	"errors"
	"fmt"
	"syscall"

	"golang.org/x/sys/windows"
)

// Package-level LazyDLL handles. Declaring them once here avoids the
// NewLazySystemDLL boilerplate previously spread across injector,
// filemanager, and crypto.
var (
	Kernel32 = windows.NewLazySystemDLL("kernel32.dll")
	Ntdll    = windows.NewLazySystemDLL("ntdll.dll")
	Crypt32  = windows.NewLazySystemDLL("crypt32.dll")
	User32   = windows.NewLazySystemDLL("user32.dll")
)

// CallBoolErr wraps the common "r1 == 0 means failure" Win32 convention.
// Win32 GetLastError often returns ERROR_SUCCESS (errno 0) even on failure,
// so we distinguish the "no-errno" case explicitly to avoid emitting a
// misleading "operation completed successfully" message. errors.As is
// used instead of a type assertion so the check stays correct if
// x/sys/windows ever wraps the underlying errno.
func CallBoolErr(p *windows.LazyProc, args ...uintptr) (uintptr, error) {
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
