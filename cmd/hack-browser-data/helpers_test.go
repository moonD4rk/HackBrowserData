package main

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

// setHome sets USERPROFILE too, because Windows reads it rather than HOME.
func setHome(t *testing.T, home string) {
	t.Helper()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
}

func runCmd(t *testing.T, command *cobra.Command, args ...string) {
	t.Helper()
	command.SetArgs(args)
	require.NoError(t, command.Execute())
}
