//go:build !darwin

package app

import "context"

func newLoginKeychain(string, func(context.Context) (string, error)) loginKeychain { return nil }

func loginKeychainPath(string) string { return "" }
