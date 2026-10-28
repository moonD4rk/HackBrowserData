//go:build !windows

package app

import "golang.org/x/sys/unix"

func canCreateIn(dir string) error {
	return unix.Access(dir, unix.W_OK|unix.X_OK)
}
