//go:build windows

package winapi

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows/registry"
)

func writeExe(t *testing.T, parts ...string) string {
	t.Helper()
	path := filepath.Join(parts...)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte("MZ"), 0o600))
	return path
}

func appPathsAt(byRoot map[registry.Key]string) func(string, registry.Key) (string, error) {
	return func(_ string, root registry.Key) (string, error) {
		if p, ok := byRoot[root]; ok {
			return p, nil
		}
		return "", errors.New("no App Paths entry")
	}
}

func runningAt(path string) func(string) string { return func(string) string { return path } }

// CocCoc and Yandex both ship browser.exe, and App Paths names only one of them.
func TestExecutablePath_InstallLocationWinsOverAnotherVendorsAppPaths(t *testing.T) {
	dir := t.TempDir()
	own := writeExe(t, dir, "CocCoc", "Browser", "Application", "browser.exe")
	other := writeExe(t, dir, "Yandex", "YandexBrowser", "Application", "browser.exe")

	got, err := executablePath("browser.exe", []string{filepath.Join(dir, "missing", "browser.exe"), own},
		appPathsAt(map[registry.Key]string{registry.LOCAL_MACHINE: other}), runningAt(other))
	require.NoError(t, err)
	assert.Equal(t, own, got)
}

func TestExecutablePath_ExpandsFallbackVariables(t *testing.T) {
	dir := t.TempDir()
	own := writeExe(t, dir, "Chrome", "chrome.exe")
	t.Setenv("HBD_TEST_INSTALL", dir)

	got, err := executablePath("chrome.exe", []string{`%HBD_TEST_INSTALL%\Chrome\chrome.exe`}, appPathsAt(nil), runningAt(""))
	require.NoError(t, err)
	assert.Equal(t, own, got)
}

func TestExecutablePath_InstallsElsewhereFallBackInOrder(t *testing.T) {
	dir := t.TempDir()
	machine := writeExe(t, dir, "machine", "chrome.exe")
	user := writeExe(t, dir, "user", "chrome.exe")
	running := writeExe(t, dir, "portable", "chrome.exe")
	missing := []string{filepath.Join(dir, "missing", "chrome.exe")}

	got, err := executablePath("chrome.exe", missing,
		appPathsAt(map[registry.Key]string{registry.LOCAL_MACHINE: machine, registry.CURRENT_USER: user}), runningAt(running))
	require.NoError(t, err)
	assert.Equal(t, machine, got)

	got, err = executablePath("chrome.exe", missing, appPathsAt(map[registry.Key]string{registry.CURRENT_USER: user}), runningAt(running))
	require.NoError(t, err)
	assert.Equal(t, user, got)

	got, err = executablePath("chrome.exe", missing, appPathsAt(nil), runningAt(running))
	require.NoError(t, err)
	assert.Equal(t, running, got)
}

func TestExecutablePath_NotFound(t *testing.T) {
	_, err := executablePath("chrome.exe", nil, appPathsAt(nil), runningAt(""))
	require.ErrorIs(t, err, ErrExecutableNotFound)

	_, err = executablePath("", nil, appPathsAt(nil), runningAt(""))
	require.ErrorIs(t, err, ErrExecutableNotFound)
}
