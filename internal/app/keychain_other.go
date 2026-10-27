//go:build !darwin

package app

import "github.com/moond4rk/hackbrowserdata/internal/masterkey"

func loginKeychain(Request) masterkey.LoginKeychain { return nil }
