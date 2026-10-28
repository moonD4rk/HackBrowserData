package main

import (
	"bytes"
	"fmt"
	"path/filepath"
	"runtime"
	"testing"
	"text/tabwriter"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/browser"
	"github.com/moond4rk/hackbrowserdata/browser/chromium"
	"github.com/moond4rk/hackbrowserdata/browser/firefox"
	"github.com/moond4rk/hackbrowserdata/browser/safari"
	ibrowser "github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/category"
	"github.com/moond4rk/hackbrowserdata/types"
)

// list prints what today's code printed on the same machine, its counts included; the comparison goes with the old
// packages.
func TestListMatchesToday(t *testing.T) {
	home := t.TempDir()
	setHome(t, home)
	for _, inst := range chromiumFixtureInstalls {
		chromiumFixtureUserData(t, userDataDirOf(t, home, inst.key), inst.key)
	}
	firefoxFixtureUserData(t, userDataDirOf(t, home, "firefox"))
	if runtime.GOOS == "darwin" {
		safariFixtureLibrary(t, filepath.Join(home, "Library"))
	}
	today := todayBrowsers(t, home)
	require.NotEmpty(t, today)

	for _, detail := range []bool{false, true} {
		var out bytes.Buffer
		cmd := listCmd()
		cmd.SetOut(&out)
		args := []string{}
		if detail {
			args = append(args, "--detail")
		}
		runCmd(t, cmd, args...)
		assert.Equalf(t, todayList(t, today, detail), out.String(), "--detail=%v", detail)
	}
}

func userDataDirOf(t *testing.T, home, id string) string {
	t.Helper()
	def, ok := ibrowser.Lookup(id)
	require.True(t, ok, id)
	return filepath.Join(home, filepath.FromSlash(def.Platform().UserDataDirs[0]))
}

// todayBrowsers finds the fixture's browsers as today's discovery did: each table row's directory under home, kept
// when its base finds a profile there.
func todayBrowsers(t *testing.T, home string) []browser.Browser {
	t.Helper()
	var out []browser.Browser
	for _, def := range ibrowser.Supported() {
		cfg := types.BrowserConfig{Key: def.ID, Name: def.Name, UserDataDir: home + "/" + def.Platform().UserDataDirs[0]}
		var (
			b   browser.Browser
			err error
		)
		switch def.Base {
		case ibrowser.Chromium:
			cfg.Kind = types.Chromium
			var c *chromium.Browser
			if c, err = chromium.NewBrowser(cfg); c != nil {
				b = c
			}
		case ibrowser.Firefox:
			cfg.Kind = types.Firefox
			var f *firefox.Browser
			if f, err = firefox.NewBrowser(cfg); f != nil {
				b = f
			}
		case ibrowser.Safari:
			cfg.Kind = types.Safari
			var s *safari.Browser
			if s, err = safari.NewBrowser(cfg); s != nil {
				b = s
			}
		}
		require.NoError(t, err, def.ID)
		if b != nil {
			out = append(out, b)
		}
	}
	return out
}

// todayList prints as today's list command did.
func todayList(t *testing.T, browsers []browser.Browser, detail bool) string {
	t.Helper()
	var buf bytes.Buffer
	w := tabwriter.NewWriter(&buf, 0, 0, 3, ' ', 0)
	if !detail {
		fmt.Fprintln(w, "Browser\tProfile\tPath")
		for _, b := range browsers {
			for _, p := range b.Profiles() {
				fmt.Fprintf(w, "%s\t%s\t%s\n", b.BrowserName(), p.Name, p.Dir)
			}
		}
		require.NoError(t, w.Flush())
		return buf.String()
	}
	fmt.Fprint(w, "Browser\tProfile")
	for _, c := range category.All() {
		fmt.Fprintf(w, "\t%s", c.String())
	}
	fmt.Fprintln(w)
	for _, b := range browsers {
		results, err := b.CountEntries(category.All())
		require.NoError(t, err)
		for _, r := range results {
			fmt.Fprintf(w, "%s\t%s", b.BrowserName(), r.Name)
			for _, c := range category.All() {
				fmt.Fprintf(w, "\t%d", r.Counts[c])
			}
			fmt.Fprintln(w)
		}
	}
	require.NoError(t, w.Flush())
	return buf.String()
}
