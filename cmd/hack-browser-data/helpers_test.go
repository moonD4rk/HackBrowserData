package main

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

// Windows reads USERPROFILE rather than HOME, so both point at the fixture.
func setHome(t *testing.T, home string) {
	t.Helper()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
}

func runCmd(t *testing.T, cmd *cobra.Command, args ...string) {
	t.Helper()
	cmd.SetArgs(args)
	require.NoError(t, cmd.Execute())
}
