//go:build !darwin

package app

func newLoginKeychain(Options) loginKeychain { return nil }
