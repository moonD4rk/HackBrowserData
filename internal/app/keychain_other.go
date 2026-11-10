//go:build !darwin

package app

func newLoginKeychain(KeychainConfig) loginKeychain { return nil }

func loginKeychainPath(string) string { return "" }
