package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/moond4rk/hackbrowserdata/internal/app"
	"github.com/moond4rk/hackbrowserdata/internal/export"
	"github.com/moond4rk/hackbrowserdata/internal/log"
)

func newRootCommand() *cobra.Command {
	var verbose bool
	root := &cobra.Command{
		Use:   "hack-browser-data",
		Short: "A CLI tool for decrypting and exporting browser data",
		Long: `hack-browser-data decrypts and exports browser data from Chromium-based
browsers, Firefox, and Safari on Windows, macOS, and Linux.

GitHub: https://github.com/moonD4rk/HackBrowserData`,
		PersistentPreRun: func(command *cobra.Command, args []string) {
			if verbose {
				log.SetVerbose()
			}
		},
	}

	root.CompletionOptions.HiddenDefaultCmd = true

	root.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable debug logging")

	dump := newDumpCommand()
	root.AddCommand(dump, newDumpKeysCommand(), newArchiveCommand(), newRestoreCommand(), newListCommand(), newVersionCommand())

	// Without a subcommand the root runs dump, as v1.1.0 did, so it carries dump's flags too.
	root.RunE = func(command *cobra.Command, args []string) error {
		dump.SetContext(command.Context())
		return dump.RunE(dump, args)
	}
	dump.Flags().VisitAll(func(flag *pflag.Flag) {
		if root.Flags().Lookup(flag.Name) == nil {
			root.Flags().AddFlag(flag)
		}
	})

	for _, command := range root.Commands() {
		silenceWhenStopped(command)
	}
	silenceWhenStopped(root)
	return root
}

// silenceWhenStopped keeps a stopped run's error and usage off the screen; main then ends the process by the signal.
func silenceWhenStopped(command *cobra.Command) {
	run := command.RunE
	if run == nil {
		return
	}
	command.RunE = func(invoked *cobra.Command, args []string) error {
		err := run(invoked, args)
		if err != nil && invoked.Context().Err() != nil {
			invoked.SilenceErrors, invoked.SilenceUsage = true, true
		}
		return err
	}
}

// report prints the run's summary; it is not part of the output, so failing to print it never fails the run.
func report(writer io.Writer, summary app.Summary) {
	if err := app.Report(writer, summary); err != nil {
		log.Warnf("print summary: %v", err)
	}
}

// foundNothing reports discovery's failures, the likely reason it found no browser data, then says so.
func foundNothing(stderr io.Writer, discovery app.Discovery, message string) bool {
	if len(discovery.Dirs) > 0 {
		return false
	}
	if len(discovery.DirFailures) > 0 {
		report(stderr, app.Summary{DirFailures: discovery.DirFailures})
	}
	log.Warnf("%s", message)
	return true
}

// announceExport lists the files a run wrote, even when it stopped early, so the user finds what it left.
func announceExport(stderr io.Writer, exported export.Output) {
	if len(exported.Files) == 0 {
		return
	}
	fmt.Fprintln(stderr)
	log.Infof("Exported to %s/", exported.Dir)
	for _, file := range exported.Files {
		log.Infof("  %-24s %d entries", file.Name, file.Count)
	}
	if exported.ZipName != "" {
		log.Infof("Compressed: %s/%s", exported.Dir, exported.ZipName)
	}
}

// stopSignals end the process by default; each first cancels the run, so its temporary copies are removed.
var stopSignals = []os.Signal{os.Interrupt, syscall.SIGTERM, syscall.SIGHUP}

type stopError struct{ received os.Signal }

func (e stopError) Error() string { return "stopped by " + e.received.String() }

func main() {
	configureDoubleClickMode()
	ctx, cancel := context.WithCancelCause(context.Background())
	signals := make(chan os.Signal, 1)
	for _, received := range stopSignals {
		if !signal.Ignored(received) { // nohup and background jobs keep ignoring what they ignore today
			signal.Notify(signals, received)
		}
	}
	go func() {
		received := <-signals
		signal.Stop(signals) // a second signal ends the process at once
		if received == os.Interrupt {
			log.Warnf("Interrupted; cleaning up (press Ctrl-C again to quit at once)")
		} else {
			log.Warnf("Stopping; cleaning up")
		}
		cancel(stopError{received})
	}()
	err := newRootCommand().ExecuteContext(ctx)
	var stop stopError
	if errors.As(context.Cause(ctx), &stop) {
		exitBy(stop.received)
	}
	if err != nil {
		os.Exit(1)
	}
}
