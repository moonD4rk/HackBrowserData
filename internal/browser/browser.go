// Package browser holds the browser table and the types the base packages hand the pipeline.
package browser

import (
	"fmt"
	"sort"

	"github.com/moond4rk/hackbrowserdata/internal/masterkey"
)

// Browser is one browser HBD knows. Name is the -b value and the keys.json vault key; DisplayName goes on every record.
type Browser struct {
	Name        string
	DisplayName string
	Base        Base

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

// Platform is one OS's half of a Browser; one without UserDataDirs means the browser is not supported there.
type Platform struct {
	UserDataDirs        []string // slash-separated, relative to the home directory; globs allowed
	SecretLabel         string   // macOS Keychain account / Linux Secret Service label; "" = none
	Executable          string   // Windows: the program the ABE payload is injected into
	ExecutableFallbacks []string // Windows: program paths tried when the registry has no App Paths entry
	ABE                 masterkey.ABEKind
}

// Supported returns the browsers of the running OS, in table order.
func Supported() []Browser {
	var out []Browser
	for _, browser := range browsers {
		if len(browser.Platform().UserDataDirs) > 0 {
			out = append(out, browser)
		}
	}
	return out
}

// Names are this OS's browser names, sorted as the -b help has always listed them.
func Names() []string {
	supported := Supported()
	names := make([]string, len(supported))
	for i, browser := range supported {
		names[i] = browser.Name
	}
	sort.Strings(names)
	return names
}

// UserDataDirID numbers by path order among a browser's user data dirs, so IDs stay the same from run to run.
func (b Browser) UserDataDirID(index int) UserDataDirID {
	if index == 0 {
		return UserDataDirID(b.Name)
	}
	return UserDataDirID(fmt.Sprintf("%s-%d", b.Name, index+1))
}

// Lookup finds a browser by name in the whole table, whatever the OS, so restore resolves another host's vaults.
func Lookup(name string) (Browser, bool) {
	for _, browser := range browsers {
		if browser.Name == name {
			return browser, true
		}
	}
	return Browser{}, false
}
