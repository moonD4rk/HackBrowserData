//go:build windows

package winutil

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"

	"github.com/moond4rk/hackbrowserdata/utils/winapi"
)

// ErrExecutableNotFound is returned when a browser's executable cannot be
// located via registry App Paths or any install-location fallback.
var ErrExecutableNotFound = errors.New("browser executable not found")

// ExecutablePath resolves a browser's .exe with a 4-tier search:
//  1. Registry App Paths in HKLM
//  2. Registry App Paths in HKCU
//  3. Running-process probe — scan EnumProcesses for a match by exe name
//     and return the owner's QueryFullProcessImageName. Picks up portable
//     builds and non-standard installs that never wrote to App Paths.
//  4. Hard-coded InstallFallbacks from Table (last resort when the browser
//     is not running and the registry is missing the entry).
//
// browserKey must match an Entry in Table; keys align with
// browser.BrowserConfig.Key (for configs that set WindowsABE: true).
func ExecutablePath(browserKey string) (string, error) {
	entry, ok := Table[browserKey]
	if !ok {
		return "", fmt.Errorf("%w: %q (no lookup entry)", ErrExecutableNotFound, browserKey)
	}

	if p, err := appPathsLookup(entry.ExeName, registry.LOCAL_MACHINE); err == nil {
		return p, nil
	}
	if p, err := appPathsLookup(entry.ExeName, registry.CURRENT_USER); err == nil {
		return p, nil
	}

	if p := runningProcessPath(entry.ExeName); p != "" {
		return p, nil
	}

	for _, candidate := range entry.InstallFallbacks {
		// Use winapi.ExpandEnvString (kernel32!ExpandEnvironmentStringsW)
		// rather than os.ExpandEnv: Go stdlib only understands Unix-style
		// $VAR / ${VAR} and leaves Windows-style %VAR% untouched, which
		// would make every fallback path fail to resolve. Verified on
		// Windows 10 19044 + Go 1.20.14.
		expanded, err := winapi.ExpandEnvString(candidate)
		if err != nil {
			continue
		}
		if fileExists(expanded) {
			return expanded, nil
		}
	}

	return "", fmt.Errorf("%w: %q (registry miss, no running process, no fallback match)",
		ErrExecutableNotFound, browserKey)
}

// runningProcessPath scans live processes for one whose image filename
// matches exeName (case-insensitive) and returns the full path on the
// first hit. Errors are swallowed — this is a best-effort probe that
// yields to the hard-coded fallbacks if nothing matches.
func runningProcessPath(exeName string) string {
	pids, err := winapi.EnumProcesses()
	if err != nil {
		return ""
	}
	for _, pid := range pids {
		if pid == 0 {
			continue
		}
		h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
		if err != nil {
			continue
		}
		path, err := winapi.QueryFullProcessImageName(h)
		_ = windows.CloseHandle(h)
		if err != nil || path == "" {
			continue
		}
		// Match the leaf filename only — a substring match against the full
		// path would accept "chrome_proxy.exe" when we asked for "chrome.exe".
		if strings.EqualFold(filepath.Base(path), exeName) {
			return path
		}
	}
	return ""
}

func appPathsLookup(exeName string, root registry.Key) (string, error) {
	sub := `SOFTWARE\Microsoft\Windows\CurrentVersion\App Paths\` + exeName
	k, err := registry.OpenKey(root, sub, registry.QUERY_VALUE)
	if err != nil {
		return "", err
	}
	defer k.Close()

	v, _, err := k.GetStringValue("")
	if err != nil {
		return "", err
	}
	v = unquote(v)
	if !fileExists(v) {
		return "", fmt.Errorf("registry path does not exist: %s", v)
	}
	return filepath.Clean(v), nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func unquote(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}
