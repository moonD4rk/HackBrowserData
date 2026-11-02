package main

import (
	"bytes"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/browser/browsertest"
	"github.com/moond4rk/hackbrowserdata/internal/golden"
)

func TestListGolden(t *testing.T) {
	home := t.TempDir()
	setHome(t, home)
	var paths []string
	for _, fixture := range browsertest.ChromiumInstalls {
		dir := userDataDirOf(t, home, fixture.Name)
		browsertest.ChromiumUserData(t, dir, fixture.Name)
		paths = append(paths, dir, "<"+fixture.Name+">")
	}
	dir := userDataDirOf(t, home, "firefox")
	browsertest.FirefoxUserData(t, dir)
	paths = append(paths, dir, "<firefox>", `\`, "/")
	assertListGoldens(t, "list", strings.NewReplacer(paths...))
}

func TestListGoldenSafari(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Safari is in the browser table on macOS only")
	}
	home := t.TempDir()
	setHome(t, home)
	library := filepath.Join(home, "Library")
	browsertest.SafariLibrary(t, library)
	assertListGoldens(t, "list-safari", strings.NewReplacer(library, "<library>"))
}

// assertListGoldens compares with the fixture's paths as placeholders, so one golden serves every OS.
func assertListGoldens(t *testing.T, prefix string, paths *strings.Replacer) {
	t.Helper()
	for _, detail := range []bool{false, true} {
		var out bytes.Buffer
		command := listCmd()
		command.SetOut(&out)
		name, args := prefix+".golden", []string{}
		if detail {
			name, args = prefix+"-detail.golden", []string{"--detail"}
		}
		runCmd(t, command, args...)
		golden.Assert(t, name, paths.Replace(out.String()))
	}
}

func userDataDirOf(t *testing.T, home, browserName string) string {
	t.Helper()
	namedBrowser, ok := browser.Lookup(browserName)
	require.True(t, ok, browserName)
	return filepath.Join(home, filepath.FromSlash(namedBrowser.Platform().UserDataDirs[0]))
}
