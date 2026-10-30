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

// keychainPassword picks the run's one source of the macOS login password, read when the keychain is first opened: a
// flag, else a no-echo prompt on a terminal, else none, which leaves keychain-protected data as metadata only.
func keychainPassword(f Flags, stdin *os.File) (func(context.Context) (string, error), error) {
	switch {
	case f.KeychainPassword != "" && f.KeychainPasswordStdin:
		return nil, errors.New("--keychain-pw and --keychain-pw-stdin cannot be used together")
	case f.KeychainPassword != "":
		password := f.KeychainPassword
		return func(context.Context) (string, error) { return password, nil }, nil
	case f.KeychainPasswordStdin:
		return func(ctx context.Context) (string, error) {
			return untilDone(ctx, func() (string, error) { return firstLine(stdin) })
		}, nil
	case term.IsTerminal(int(stdin.Fd())):
		return func(ctx context.Context) (string, error) { return prompt(ctx, stdin) }, nil
	default:
		return func(context.Context) (string, error) { return "", errNoPassword }, nil
	}
}

func firstLine(r io.Reader) (string, error) {
	line, err := bufio.NewReader(r).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("read the password from stdin: %w", err)
	}
	return strings.TrimRight(line, "\r\n"), nil
}

func prompt(ctx context.Context, stdin *os.File) (string, error) {
	fd := int(stdin.Fd())
	state, err := term.GetState(fd)
	if err != nil {
		return "", fmt.Errorf("read the password: %w", err)
	}
	fmt.Fprint(os.Stderr, "Enter macOS login password: ")
	password, err := untilDone(ctx, func() (string, error) {
		p, err := term.ReadPassword(fd)
		return string(p), err
	})
	if ctx.Err() != nil {
		// The abandoned read never turns the echo back on, and the stop notice has already ended the line.
		return "", errors.Join(ctx.Err(), term.Restore(fd, state))
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
	case a := <-answers:
		return a.value, a.err
	case <-ctx.Done():
		return "", ctx.Err()
	}
}
