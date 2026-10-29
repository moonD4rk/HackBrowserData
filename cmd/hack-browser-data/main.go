package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/moond4rk/hackbrowserdata/internal/log"
)

var verbose bool

func rootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "hack-browser-data",
		Short: "A CLI tool for decrypting and exporting browser data",
		Long: `hack-browser-data decrypts and exports browser data from Chromium-based
browsers and Firefox on Windows, macOS, and Linux.

GitHub: https://github.com/moonD4rk/HackBrowserData`,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if verbose {
				log.SetVerbose()
			}
		},
	}

	root.CompletionOptions.HiddenDefaultCmd = true

	root.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable debug logging")

	dump := dumpCmd()
	root.AddCommand(dump, dumpKeysCmd(), archiveCmd(), restoreCmd(), listCmd(), versionCmd())

	// Default to dump when no subcommand is given.
	// Copy dump flags to root so that `hack-browser-data -b chrome`
	// works the same as `hack-browser-data dump -b chrome`.
	root.RunE = func(cmd *cobra.Command, args []string) error {
		dump.SetContext(cmd.Context())
		return dump.RunE(dump, args)
	}
	dump.Flags().VisitAll(func(f *pflag.Flag) {
		if root.Flags().Lookup(f.Name) == nil {
			root.Flags().AddFlag(f)
		}
	})

	for _, c := range root.Commands() {
		quietWhenStopped(c)
	}
	quietWhenStopped(root)
	return root
}

// quietWhenStopped keeps a stopped run's error and usage off the screen; main then ends the process by the signal.
func quietWhenStopped(c *cobra.Command) {
	run := c.RunE
	if run == nil {
		return
	}
	c.RunE = func(cmd *cobra.Command, args []string) error {
		err := run(cmd, args)
		if err != nil && cmd.Context().Err() != nil {
			cmd.SilenceErrors, cmd.SilenceUsage = true, true
		}
		return err
	}
}

// stopSignals end the process by default; each first cancels the run, so its temporary copies are removed.
var stopSignals = []os.Signal{os.Interrupt, syscall.SIGTERM, syscall.SIGHUP}

type stopError struct{ sig os.Signal }

func (e stopError) Error() string { return "stopped by " + e.sig.String() }

func main() {
	configureDoubleClickMode()
	ctx, cancel := context.WithCancelCause(context.Background())
	signals := make(chan os.Signal, 1)
	for _, sig := range stopSignals {
		if !signal.Ignored(sig) { // nohup and background jobs keep ignoring what they ignore today
			signal.Notify(signals, sig)
		}
	}
	go func() {
		sig := <-signals
		signal.Stop(signals) // a second signal ends the process at once
		if sig == os.Interrupt {
			log.Warnf("Interrupted; cleaning up (press Ctrl-C again to quit at once)")
		} else {
			log.Warnf("Stopping; cleaning up")
		}
		cancel(stopError{sig})
	}()
	err := rootCmd().ExecuteContext(ctx)
	var stop stopError
	if errors.As(context.Cause(ctx), &stop) {
		exitBy(stop.sig)
	}
	if err != nil {
		os.Exit(1)
	}
}
