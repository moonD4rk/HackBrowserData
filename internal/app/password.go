package app

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

var errNoPassword = errors.New("macOS login password not given and stdin is not a terminal")

// keychainPasswordFunc is read lazily: a flag, else a no-echo prompt on a terminal, else none (metadata only).
func keychainPasswordFunc(flags KeychainFlags, stdin *os.File) (func(context.Context) (string, error), error) {
	switch {
	case flags.Password != "" && flags.PasswordStdin:
		return nil, errors.New("--keychain-pw and --keychain-pw-stdin cannot be used together")
	case flags.Password != "":
		password := flags.Password
		return func(context.Context) (string, error) { return password, nil }, nil
	case flags.PasswordStdin:
		return func(ctx context.Context) (string, error) {
			return untilDone(ctx, func() (string, error) { return firstLine(stdin) })
		}, nil
	case term.IsTerminal(int(stdin.Fd())):
		return func(ctx context.Context) (string, error) { return promptPassword(ctx, stdin) }, nil
	default:
		return func(context.Context) (string, error) { return "", errNoPassword }, nil
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
