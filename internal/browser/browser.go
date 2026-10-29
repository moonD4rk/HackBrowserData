// Package browser holds the browser table and what the base packages hand the pipeline: user data dirs, profiles,
// and the extractors that read them.
package browser

import "github.com/moond4rk/hackbrowserdata/internal/masterkey"

// Def is one browser HBD knows. ID is the -b value and the keys.json vault key; Name is stamped on every record.
type Def struct {
	ID   string
	Name string
	Base Base

	Darwin  Platform
	Linux   Platform
	Windows Platform
}

// Base is what a browser is built on; it selects the base package and the key path.
type Base int

const (
	Chromium Base = iota + 1
	Firefox
	Safari
)

// String is the base's name in a snapshot manifest.
func (b Base) String() string {
	switch b {
	case Chromium:
		return "chromium"
	case Firefox:
		return "firefox"
	case Safari:
		return "safari"
	default:
		return "unknown"
	}
}

// Platform is how one browser appears on one OS; a Platform without UserDataDirs means the OS is not supported.
type Platform struct {
	UserDataDirs  []string // slash-separated, relative to the home directory; globs allowed
	KeychainLabel string   // macOS Keychain account / Linux Secret Service label; "" = none
	Exe           string   // Windows: the program the ABE payload is injected into
	ExeFallbacks  []string // Windows: program paths tried when the registry has no App Paths entry
	ABE           masterkey.ABEKind
}

// Supported returns the browsers of the running OS, in table order.
func Supported() []Def {
	var out []Def
	for _, d := range defs {
		if len(d.Platform().UserDataDirs) > 0 {
			out = append(out, d)
		}
	}
	return out
}

// Lookup finds a browser by ID in the whole table, whatever the running OS, so restore resolves another host's vaults.
func Lookup(id string) (Def, bool) {
	for _, d := range defs {
		if d.ID == id {
			return d, true
		}
	}
	return Def{}, false
}
