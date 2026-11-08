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
)

// keychainFlags say where the macOS login keychain password comes from; both empty means a prompt, or none.
type keychainFlags struct {
	password      string
	passwordStdin bool
}

func addKeychainPasswordFlags(command *cobra.Command, flags *keychainFlags) {
	command.Flags().StringVar(&flags.password, "keychain-pw", "", "macOS keychain password (visible in ps and shell history)")
	command.Flags().BoolVar(&flags.passwordStdin, "keychain-pw-stdin", false,
		"read the macOS keychain password from the first line of stdin")
	command.MarkFlagsMutuallyExclusive("keychain-pw", "keychain-pw-stdin")
}

var errNoPassword = errors.New("macOS login password not given and stdin is not a terminal")

// keychainPasswordFunc is read lazily: a flag, else a no-echo prompt on a terminal, else none (metadata only).
func keychainPasswordFunc(flags keychainFlags, stdin *os.File) func(context.Context) (string, error) {
	switch {
	case flags.password != "":
		password := flags.password
		return func(context.Context) (string, error) { return password, nil }
	case flags.passwordStdin:
		return func(ctx context.Context) (string, error) {
			return untilDone(ctx, func() (string, error) { return firstLine(stdin) })
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
	password, err := untilDone(ctx, func() (string, error) {
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

// untilDone stops waiting when the run is stopped; the read itself stays blocked until the process ends.
func untilDone(ctx context.Context, read func() (string, error)) (string, error) {
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
