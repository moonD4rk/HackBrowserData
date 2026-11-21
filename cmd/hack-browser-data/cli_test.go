package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/internal/golden"
	"github.com/moond4rk/hackbrowserdata/internal/masterkey"
)

// browsersPlaceholder stands for this OS's browser names, so one golden serves every OS.
const browsersPlaceholder = "<browsers>"

func TestHelpGolden(t *testing.T) {
	var screens strings.Builder
	for _, args := range [][]string{
		{"-h"}, {"dump", "-h"}, {"dumpkeys", "-h"}, {"archive", "-h"}, {"restore", "-h"}, {"list", "-h"}, {"version", "-h"},
	} {
		_, screen, err := runRoot(args...)
		require.NoError(t, err, args)
		screens.WriteString(commandLine(args) + screen)
	}
	golden.Assert(t, "help.golden", screens.String())
}

// TestUsageErrorsGolden puts a marker where a run printed its usage: help.golden pins the wording, this pins when it shows.
func TestUsageErrorsGolden(t *testing.T) {
	setHome(t, t.TempDir())
	var screens strings.Builder
	for _, args := range [][]string{
		{"-b", "nosuch"},
		{"dump", "-b", "nosuch"},
		{"dump", "-c", "nosuch"},
		{"dump", "-f", "xml"},
		{"dump", "--max-storage-value", "-1"},
		{"dump", "--keychain-pw", "secret", "--keychain-pw-stdin"},
		{"dumpkeys", "-b", "nosuch"},
		{"archive", "-c", "nosuch"},
		{"restore"},
		{"restore", "--keys", "keys.json"},
		{"restore", "--keys", "keys.json", "--data-dir", "data", "--data-zip", "data.zip"},
		{"list", "--nosuch"},
		{"nosuch"},
	} {
		root, screen, err := runRoot(args...)
		require.Error(t, err, args)
		for _, command := range sortedCommands(root) {
			usage := strings.ReplaceAll(command.UsageString(), browserNames(), browsersPlaceholder)
			screen = strings.Replace(screen, usage, "<usage: "+command.CommandPath()+">\n", 1)
		}
		screens.WriteString(commandLine(args) + screen)
	}
	golden.Assert(t, "errors.golden", screens.String())
}

func TestNothingFound(t *testing.T) {
	setHome(t, t.TempDir())

	t.Run("dump creates no output dir", func(t *testing.T) {
		outputDir := filepath.Join(t.TempDir(), "results")
		_, _, err := runRoot("dump", "-d", outputDir)
		require.NoError(t, err)
		assert.NoDirExists(t, outputDir)
	})

	t.Run("archive creates no archive", func(t *testing.T) {
		archivePath := filepath.Join(t.TempDir(), "browser-data.zip")
		_, _, err := runRoot("archive", "-o", archivePath)
		require.NoError(t, err)
		assert.NoFileExists(t, archivePath)
	})

	t.Run("list says so", func(t *testing.T) {
		_, screen, err := runRoot("list")
		require.NoError(t, err)
		assert.Equal(t, "No browsers found.\n", screen)
	})

	t.Run("dumpkeys prints a key file without vaults", func(t *testing.T) {
		_, screen, err := runRoot("dumpkeys")
		require.NoError(t, err)
		keyFile, err := masterkey.ReadKeyFile(strings.NewReader(screen))
		require.NoError(t, err)
		assert.Empty(t, keyFile.Vaults)
	})
}

// runRoot returns the root it ran, for its commands' usage texts, and everything the run printed.
func runRoot(args ...string) (*cobra.Command, string, error) {
	var screen bytes.Buffer
	root := newRootCommand()
	root.SetOut(&screen)
	root.SetErr(&screen)
	root.SetArgs(args)
	err := root.Execute()
	return root, strings.ReplaceAll(screen.String(), browserNames(), browsersPlaceholder), err
}

func commandLine(args []string) string {
	return "$ hack-browser-data " + strings.Join(args, " ") + "\n"
}
