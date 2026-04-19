//go:build windows

package winapi

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Call-style procs used by the typed wrappers below.
var (
	procVirtualAllocEx     = Kernel32.NewProc("VirtualAllocEx")
	procCreateRemoteThread = Kernel32.NewProc("CreateRemoteThread")

	// K32EnumProcesses is the kernel32-embedded twin of psapi!EnumProcesses
	// introduced in Windows 7 — using it lets us skip the psapi.dll handle.
	procK32EnumProcesses          = Kernel32.NewProc("K32EnumProcesses")
	procQueryFullProcessImageName = Kernel32.NewProc("QueryFullProcessImageNameW")
)

// Address-style procs. The injector reads their raw addresses via .Addr()
// and patches them into the reflective loader's DOS stub. We never Call
// these from our own process.
var (
	procLoadLibraryA   = Kernel32.NewProc("LoadLibraryA")
	procGetProcAddress = Kernel32.NewProc("GetProcAddress")
	procVirtualAlloc   = Kernel32.NewProc("VirtualAlloc")
	procVirtualProtect = Kernel32.NewProc("VirtualProtect")
	procNtFlushIC      = Ntdll.NewProc("NtFlushInstructionCache")
)

// VirtualAllocEx wraps kernel32!VirtualAllocEx. Returns the allocated
// base address in the target process, or an error surfacing Win32
// errno-0 explicitly via CallBoolErr.
func VirtualAllocEx(proc windows.Handle, size uintptr, flAllocType, flProtect uint32) (uintptr, error) {
	return CallBoolErr(procVirtualAllocEx,
		uintptr(proc), 0, size,
		uintptr(flAllocType), uintptr(flProtect),
	)
}

// CreateRemoteThread wraps kernel32!CreateRemoteThread. Returns the new
// thread's handle, which the caller must CloseHandle.
func CreateRemoteThread(proc windows.Handle, startAddr, param uintptr) (windows.Handle, error) {
	h, err := CallBoolErr(procCreateRemoteThread,
		uintptr(proc), 0, 0,
		startAddr, param, 0, 0,
	)
	if err != nil {
		return 0, err
	}
	return windows.Handle(h), nil
}

// Addr* functions expose raw function pointers for the reflective
// loader's DOS-stub patching. KnownDlls + session-consistent ASLR
// guarantees these addresses are valid in every process spawned in
// the same boot session.

func AddrLoadLibraryA() uintptr            { return procLoadLibraryA.Addr() }
func AddrGetProcAddress() uintptr          { return procGetProcAddress.Addr() }
func AddrVirtualAlloc() uintptr            { return procVirtualAlloc.Addr() }
func AddrVirtualProtect() uintptr          { return procVirtualProtect.Addr() }
func AddrNtFlushInstructionCache() uintptr { return procNtFlushIC.Addr() }

// EnumProcesses returns all PIDs currently visible to the caller. Backed
// by kernel32!K32EnumProcesses (available on Windows 7+), so we do not
// need a separate psapi.dll handle. The buffer doubles on overflow up to
// a 1M-entry safety cap.
func EnumProcesses() ([]uint32, error) {
	size := uint32(1024)
	for {
		pids := make([]uint32, size)
		var bytesReturned uint32
		r, _, err := procK32EnumProcesses.Call(
			uintptr(unsafe.Pointer(&pids[0])),
			uintptr(size*4),
			uintptr(unsafe.Pointer(&bytesReturned)),
		)
		if r == 0 {
			return nil, fmt.Errorf("K32EnumProcesses: %w", err)
		}
		n := int(bytesReturned / 4)
		// A completely filled buffer means we may have truncated — grow and retry.
		if n < int(size) {
			return pids[:n], nil
		}
		size *= 2
		if size > 1<<20 {
			return nil, fmt.Errorf("EnumProcesses: PID buffer exceeded 1M entries")
		}
	}
}

// QueryFullProcessImageName returns the full file-system path of the
// executable backing the given process handle. Open the handle with
// PROCESS_QUERY_LIMITED_INFORMATION (available to non-admin callers).
func QueryFullProcessImageName(h windows.Handle) (string, error) {
	buf := make([]uint16, windows.MAX_PATH)
	size := uint32(len(buf))
	r, _, err := procQueryFullProcessImageName.Call(
		uintptr(h),
		0,
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&size)),
	)
	if r == 0 {
		return "", fmt.Errorf("QueryFullProcessImageNameW: %w", err)
	}
	return windows.UTF16ToString(buf[:size]), nil
}
