package main

import (
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/moond4rk/hackbrowserdata/internal/golden"
)

func TestFlagSurface(t *testing.T) {
	golden.Assert(t, "flags.golden", flagSurface(rootCmd()))
}

func flagSurface(root *cobra.Command) string {
	var lines []string
	for _, command := range sortedCommands(root) {
		command.InitDefaultHelpFlag()
		path := command.CommandPath()
		for _, flag := range sortedFlags(command.LocalFlags()) {
			marker := ""
			if command.PersistentFlags().Lookup(flag.Name) == flag {
				marker = "persistent"
			}
			lines = append(lines, flagLine(path, flag, marker))
		}
		for _, flag := range sortedFlags(command.InheritedFlags()) {
			lines = append(lines, flagLine(path, flag, "inherited"))
		}
	}
	return strings.Join(lines, "\n") + "\n"
}

func sortedCommands(root *cobra.Command) []*cobra.Command {
	commands := []*cobra.Command{root}
	for i := 0; i < len(commands); i++ {
		commands = append(commands, commands[i].Commands()...)
	}
	sort.Slice(commands, func(i, j int) bool { return commands[i].CommandPath() < commands[j].CommandPath() })
	return commands
}

func sortedFlags(flagSet *pflag.FlagSet) []*pflag.Flag {
	var flags []*pflag.Flag
	flagSet.VisitAll(func(flag *pflag.Flag) { flags = append(flags, flag) })
	sort.Slice(flags, func(i, j int) bool { return flags[i].Name < flags[j].Name })
	return flags
}

func flagLine(path string, flag *pflag.Flag, marker string) string {
	// Usage text is excluded: its wording may still be clarified, while name, shorthand, type and default are frozen.
	fields := []string{path, "--" + flag.Name}
	if flag.Shorthand != "" {
		fields = append(fields, "-"+flag.Shorthand)
	}
	fields = append(fields, flag.Value.Type(), strconv.Quote(flag.DefValue))
	if marker != "" {
		fields = append(fields, marker)
	}
	return strings.Join(fields, " ")
}
