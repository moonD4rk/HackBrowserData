package main

import (
	"flag"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var update = flag.Bool("update", false, "rewrite testdata/flags.golden from the current command tree")

func TestFlagSurface(t *testing.T) {
	golden := filepath.Join("testdata", "flags.golden")
	got := flagSurface(rootCmd())
	if *update {
		require.NoError(t, os.MkdirAll(filepath.Dir(golden), 0o755))
		require.NoError(t, os.WriteFile(golden, []byte(got), 0o644))
	}
	want, err := os.ReadFile(golden)
	require.NoError(t, err)
	assert.Equal(t, string(want), got, "regenerate with: go test ./cmd/hack-browser-data -run TestFlagSurface -update")
}

func flagSurface(root *cobra.Command) string {
	var lines []string
	for _, cmd := range sortedCommands(root) {
		cmd.InitDefaultHelpFlag()
		path := cmd.CommandPath()
		for _, f := range sortedFlags(cmd.LocalFlags()) {
			marker := ""
			if cmd.PersistentFlags().Lookup(f.Name) == f {
				marker = "persistent"
			}
			lines = append(lines, flagLine(path, f, marker))
		}
		for _, f := range sortedFlags(cmd.InheritedFlags()) {
			lines = append(lines, flagLine(path, f, "inherited"))
		}
	}
	return strings.Join(lines, "\n") + "\n"
}

func sortedCommands(root *cobra.Command) []*cobra.Command {
	cmds := []*cobra.Command{root}
	for i := 0; i < len(cmds); i++ {
		cmds = append(cmds, cmds[i].Commands()...)
	}
	sort.Slice(cmds, func(i, j int) bool { return cmds[i].CommandPath() < cmds[j].CommandPath() })
	return cmds
}

func sortedFlags(fs *pflag.FlagSet) []*pflag.Flag {
	var flags []*pflag.Flag
	fs.VisitAll(func(f *pflag.Flag) { flags = append(flags, f) })
	sort.Slice(flags, func(i, j int) bool { return flags[i].Name < flags[j].Name })
	return flags
}

func flagLine(path string, f *pflag.Flag, marker string) string {
	// Usage text is excluded: its wording may still be clarified, while name, shorthand, type and default are frozen.
	fields := []string{path, "--" + f.Name}
	if f.Shorthand != "" {
		fields = append(fields, "-"+f.Shorthand)
	}
	fields = append(fields, f.Value.Type(), strconv.Quote(f.DefValue))
	if marker != "" {
		fields = append(fields, marker)
	}
	return strings.Join(fields, " ")
}
