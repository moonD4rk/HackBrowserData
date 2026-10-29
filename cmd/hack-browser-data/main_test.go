package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/internal/browser/browsertest"
)

func TestStoppedRunPrintsNoErrorOrUsage(t *testing.T) {
	home := t.TempDir()
	setHome(t, home)
	browsertest.ChromiumUserData(t, userDataDirOf(t, home, "chrome"), "chrome")

	for _, tc := range []struct {
		name, browser string
		stop          bool
	}{
		{name: "stopped", browser: "chrome", stop: true},
		{name: "failed", browser: "nosuch"},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if tc.stop {
				cancel()
			}
			var screen bytes.Buffer
			root := rootCmd()
			root.SetOut(&screen)
			root.SetErr(&screen)
			root.SetArgs([]string{"-b", tc.browser, "-d", filepath.Join(t.TempDir(), "results")})

			err := root.ExecuteContext(ctx)
			require.Error(t, err)
			if tc.stop {
				require.ErrorIs(t, err, context.Canceled)
				assert.Empty(t, screen.String())
			} else {
				assert.Contains(t, screen.String(), "Error: ")
			}
		})
	}
}

func TestExitByEndsTheProcessAsTheSignalWould(t *testing.T) {
	if os.Getenv("HBD_TEST_EXIT_BY") == "1" {
		exitBy(os.Interrupt)
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestExitByEndsTheProcessAsTheSignalWould$")
	cmd.Env = append(os.Environ(), "HBD_TEST_EXIT_BY=1")
	var exitErr *exec.ExitError
	require.ErrorAs(t, cmd.Run(), &exitErr)
	assertEndedByInterrupt(t, exitErr.ProcessState)
}
