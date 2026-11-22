//go:build windows

package app

// checkWritable accepts every directory: Windows grants writes through ACLs that only the write itself can check.
func checkWritable(string) error { return nil }
