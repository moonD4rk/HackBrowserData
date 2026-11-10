package app

import (
	"context"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/category"
	"github.com/moond4rk/hackbrowserdata/internal/record"
)

type ListOptions struct {
	Target   Target
	Keychain KeychainConfig
}

// ValidateList targets every browser and leaves the keychain without a password, so counting never prompts.
func ValidateList() (ListOptions, error) {
	home, err := homeDir()
	if err != nil {
		return ListOptions{}, err
	}
	return ListOptions{
		Target:   Target{Browsers: browser.Supported(), HomeDir: home},
		Keychain: KeychainConfig{Path: loginKeychainPath(home)},
	}, nil
}

// CountRecords runs every category with no key and the keychain locked, counting records instead of exporting them.
func CountRecords(ctx context.Context, discovery Discovery, keychainConfig KeychainConfig) (Summary, error) {
	keychain := newLoginKeychain(keychainConfig)
	counter := runner{writer: discard{}, extractors: extractorsWith(keychain), retrieveKeys: keysFromFile(nil)}
	return counter.run(ctx, discovery, category.All())
}

type discard struct{}

func (discard) Write(category.Category, []record.Record) error { return nil }
