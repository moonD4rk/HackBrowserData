package main

import (
	"fmt"
	"runtime/debug"

	"github.com/spf13/cobra"
)

var (
	version   = "dev"
	commit    = "none"
	buildDate = "unknown"
)

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, _ []string) {
			resolveVersionFromBuildInfo()
			fmt.Fprintf(cmd.OutOrStdout(), "hack-browser-data %s\n  commit: %s\n  built:  %s\n",
				version, commit, buildDate)
		},
	}
}

func resolveVersionFromBuildInfo() {
	if version != "dev" {
		return
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}
	if info.Main.Version != "" && info.Main.Version != "(devel)" {
		version = info.Main.Version
	}
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			if len(s.Value) > 8 {
				commit = s.Value[:8]
			} else if s.Value != "" {
				commit = s.Value
			}
		case "vcs.time":
			if s.Value != "" {
				buildDate = s.Value
			}
		}
	}
}
