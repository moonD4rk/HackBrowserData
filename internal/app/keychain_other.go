//go:build !darwin

package app

import "github.com/moond4rk/hackbrowserdata/internal/keychain"

func newLoginKeychain(string, keychain.PasswordFunc) loginKeychain { return nil }

func loginKeychainPath(string) string { return "" }
