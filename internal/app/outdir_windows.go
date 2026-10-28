//go:build windows

package app

// canCreateIn accepts every directory: Windows grants writes through ACLs that only the write itself can check.
func canCreateIn(string) error { return nil }
