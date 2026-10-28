//go:build !darwin

package app

func loginKeychain(Request) runKeychain { return nil }
