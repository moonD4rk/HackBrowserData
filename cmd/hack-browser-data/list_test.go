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
	for _, inst := range browsertest.ChromiumInstalls {
		dir := userDataDirOf(t, home, inst.Key)
		browsertest.ChromiumUserData(t, dir, inst.Key)
		paths = append(paths, dir, "<"+inst.Key+">")
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

// The goldens hold the fixture's paths as placeholders, so one golden serves every OS.
func assertListGoldens(t *testing.T, prefix string, paths *strings.Replacer) {
	t.Helper()
	for _, detail := range []bool{false, true} {
		var out bytes.Buffer
		cmd := listCmd()
		cmd.SetOut(&out)
		name, args := prefix+".golden", []string{}
		if detail {
			name, args = prefix+"-detail.golden", []string{"--detail"}
		}
		runCmd(t, cmd, args...)
		golden.Assert(t, name, paths.Replace(out.String()))
	}
}

func userDataDirOf(t *testing.T, home, id string) string {
	t.Helper()
	def, ok := browser.Lookup(id)
	require.True(t, ok, id)
	return filepath.Join(home, filepath.FromSlash(def.Platform().UserDataDirs[0]))
}
