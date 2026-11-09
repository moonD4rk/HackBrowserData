package main

import (
	"context"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKeychainPasswordFunc(t *testing.T) {
	tests := []struct {
		name  string
		flags keychainFlags
		stdin string
		want  string
		err   error
	}{
		{name: "flag", flags: keychainFlags{password: "from-flag"}, stdin: "from-stdin\n", want: "from-flag"},
		{name: "stdin", flags: keychainFlags{passwordStdin: true}, stdin: "from-stdin\r\nnext\n", want: "from-stdin"},
		{name: "stdin without newline", flags: keychainFlags{passwordStdin: true}, stdin: "from-stdin", want: "from-stdin"},
		{name: "none off a terminal", stdin: "from-stdin\n", err: errNoPassword},
	}
	for _, testCase := range tests {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			pipeReader, pipeWriter, err := os.Pipe()
			require.NoError(t, err)
			defer pipeReader.Close()
			_, err = pipeWriter.WriteString(testCase.stdin)
			require.NoError(t, errors.Join(err, pipeWriter.Close()))

			got, err := keychainPasswordFunc(testCase.flags, pipeReader)(context.Background())
			require.ErrorIs(t, err, testCase.err)
			assert.Equal(t, testCase.want, got)
		})
	}
}

// TestKeychainPasswordFlagsAreExclusive covers the root command too, which runs dump with dump's flags.
func TestKeychainPasswordFlagsAreExclusive(t *testing.T) {
	setHome(t, t.TempDir())
	out := t.TempDir()
	for _, args := range [][]string{
		{"-d", out, "--keychain-pw", "pw", "--keychain-pw-stdin"},
		{"dump", "-d", out, "--keychain-pw", "pw", "--keychain-pw-stdin"},
		{"dumpkeys", "--keychain-pw", "pw", "--keychain-pw-stdin"},
	} {
		root := newRootCommand()
		root.SetOut(io.Discard)
		root.SetErr(io.Discard)
		root.SetArgs(args)
		assert.ErrorContains(t, root.Execute(), "keychain-pw", args)
	}
}
