package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/moond4rk/hackbrowserdata/internal/keychain"
)

// passwordFlags say where one password comes from: its flag, or the first line of stdin. Neither leaves it to the
// password's own default: a prompt or nothing for the keychain, the empty password for Firefox.
type passwordFlags struct {
	value     string
	fromStdin bool
}

// addPasswordFlags defines --<name> and --<name>-stdin for the password that what names.
func addPasswordFlags(command *cobra.Command, flags *passwordFlags, name, what string) {
	command.Flags().StringVar(&flags.value, name, "", what+" (visible in ps and shell history)")
	command.Flags().BoolVar(&flags.fromStdin, name+"-stdin", false, "read the "+what+" from the first line of stdin")
	command.MarkFlagsMutuallyExclusive(name, name+"-stdin")
}

// readFirefoxPassword takes the flag, else the first line of stdin; "" is the empty password, and it never prompts.
func readFirefoxPassword(ctx context.Context, flags passwordFlags, stdin io.Reader) (string, error) {
	if flags.fromStdin {
		return readContext(ctx, func() (string, error) { return firstLine(stdin) })
	}
	return flags.value, nil
}

var errNoPassword = errors.New("macOS login password not given and stdin is not a terminal")

// keychainPasswordFunc is read lazily: a flag, else a no-echo prompt on a terminal, else none (metadata only).
func keychainPasswordFunc(flags passwordFlags, stdin *os.File) keychain.PasswordFunc {
	switch {
	case flags.value != "":
		password := flags.value
		return func(context.Context) (string, error) { return password, nil }
	case flags.fromStdin:
		return func(ctx context.Context) (string, error) {
			return readContext(ctx, func() (string, error) { return firstLine(stdin) })
		}
	case term.IsTerminal(int(stdin.Fd())):
		return func(ctx context.Context) (string, error) { return promptPassword(ctx, stdin) }
	default:
		return func(context.Context) (string, error) { return "", errNoPassword }
	}
}

func firstLine(reader io.Reader) (string, error) {
	line, err := bufio.NewReader(reader).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("read the password from stdin: %w", err)
	}
	return strings.TrimRight(line, "\r\n"), nil
}

func promptPassword(ctx context.Context, stdin *os.File) (string, error) {
	fileDescriptor := int(stdin.Fd())
	state, err := term.GetState(fileDescriptor)
	if err != nil {
		return "", fmt.Errorf("read the password: %w", err)
	}
	fmt.Fprint(os.Stderr, "Enter macOS login password: ")
	password, err := readContext(ctx, func() (string, error) {
		entered, err := term.ReadPassword(fileDescriptor)
		return string(entered), err
	})
	if ctx.Err() != nil {
		// The abandoned read never turns the echo back on, and the stop notice has already ended the line.
		return "", errors.Join(ctx.Err(), term.Restore(fileDescriptor, state))
	}
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", fmt.Errorf("read the password: %w", err)
	}
	return password, nil
}

// readContext stops waiting when the run is stopped; the read itself stays blocked until the process ends.
func readContext(ctx context.Context, read func() (string, error)) (string, error) {
	type answer struct {
		value string
		err   error
	}
	answers := make(chan answer, 1)
	go func() {
		value, err := read()
		answers <- answer{value, err}
	}()
	select {
	case reply := <-answers:
		return reply.value, reply.err
	case <-ctx.Done():
		return "", ctx.Err()
	}
}
