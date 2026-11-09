//go:build !darwin

package app

func newLoginKeychain(KeychainConfig) loginKeychain { return nil }
