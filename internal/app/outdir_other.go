//go:build !windows

package app

import "golang.org/x/sys/unix"

func canCreateIn(directory string) error {
	return unix.Access(directory, unix.W_OK|unix.X_OK)
}
