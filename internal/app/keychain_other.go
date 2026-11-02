//go:build !darwin

package app

func loginKeychain(Options) runKeychain { return nil }
