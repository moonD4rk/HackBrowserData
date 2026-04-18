//go:build windows

package browserutil

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows/registry"
)

var ErrExecutableNotFound = errors.New("browser executable not found")

type browserLocation struct {
	exeName   string
	fallbacks []string
}

var browserLocations = map[string]browserLocation{
	"chrome": {
		exeName: "chrome.exe",
		fallbacks: []string{
			`%ProgramFiles%\Google\Chrome\Application\chrome.exe`,
			`%ProgramFiles(x86)%\Google\Chrome\Application\chrome.exe`,
			`%LocalAppData%\Google\Chrome\Application\chrome.exe`,
		},
	},
	"chrome-beta": {
		exeName: "chrome.exe",
		fallbacks: []string{
			`%ProgramFiles%\Google\Chrome Beta\Application\chrome.exe`,
			`%ProgramFiles(x86)%\Google\Chrome Beta\Application\chrome.exe`,
			`%LocalAppData%\Google\Chrome Beta\Application\chrome.exe`,
		},
	},
	"edge": {
		exeName: "msedge.exe",
		fallbacks: []string{
			`%ProgramFiles(x86)%\Microsoft\Edge\Application\msedge.exe`,
			`%ProgramFiles%\Microsoft\Edge\Application\msedge.exe`,
		},
	},
	"brave": {
		exeName: "brave.exe",
		fallbacks: []string{
			`%ProgramFiles%\BraveSoftware\Brave-Browser\Application\brave.exe`,
			`%ProgramFiles(x86)%\BraveSoftware\Brave-Browser\Application\brave.exe`,
			`%LocalAppData%\BraveSoftware\Brave-Browser\Application\brave.exe`,
		},
	},
	"coccoc": {
		exeName: "browser.exe",
		fallbacks: []string{
			`%ProgramFiles%\CocCoc\Browser\Application\browser.exe`,
			`%ProgramFiles(x86)%\CocCoc\Browser\Application\browser.exe`,
			`%LocalAppData%\CocCoc\Browser\Application\browser.exe`,
		},
	},
}

func ExecutablePath(browserKey string) (string, error) {
	loc, ok := browserLocations[browserKey]
	if !ok {
		return "", fmt.Errorf("%w: %q (no lookup entry)", ErrExecutableNotFound, browserKey)
	}

	if p, err := appPathsLookup(loc.exeName, registry.LOCAL_MACHINE); err == nil {
		return p, nil
	}
	if p, err := appPathsLookup(loc.exeName, registry.CURRENT_USER); err == nil {
		return p, nil
	}

	for _, candidate := range loc.fallbacks {
		expanded := os.ExpandEnv(candidate)
		if fileExists(expanded) {
			return expanded, nil
		}
	}

	return "", fmt.Errorf("%w: %q (registry miss and no fallback match)",
		ErrExecutableNotFound, browserKey)
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
