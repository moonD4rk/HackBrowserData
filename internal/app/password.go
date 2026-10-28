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
		return func(context.Context) (string, error) { return firstLine(stdin) }, nil
	case term.IsTerminal(int(stdin.Fd())):
		return func(context.Context) (string, error) { return prompt(stdin) }, nil
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

func prompt(stdin *os.File) (string, error) {
	fmt.Fprint(os.Stderr, "Enter macOS login password: ")
	password, err := term.ReadPassword(int(stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", fmt.Errorf("read the password: %w", err)
	}
	return string(password), nil
}
