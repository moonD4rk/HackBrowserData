// Package keychain owns the user's macOS login keychain: one handle per run, opened on first use.
package keychain

import "time"

// InternetPassword is one internet password item; Password is empty while the keychain stays locked.
type InternetPassword struct {
	Protocol string
	Server   string
	Port     uint32
	Path     string
	Account  string
	Password string
	Created  time.Time
}
