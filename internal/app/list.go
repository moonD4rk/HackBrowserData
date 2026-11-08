package app

import (
	"context"

	"github.com/moond4rk/hackbrowserdata/internal/category"
	"github.com/moond4rk/hackbrowserdata/internal/record"
)

// ListOptions is what list reads: every browser and category, with the keychain left locked, so counting never prompts.
func ListOptions() (Options, error) {
	home, err := homeDir()
	if err != nil {
		return Options{}, err
	}
	return Options{
		Target:     Target{BrowserName: allBrowsers, HomeDir: home},
		Categories: category.All(),
		Keychain:   KeychainConfig{HomeDir: home},
	}, nil
}

// CountRecords runs the plan with no key and the keychain locked, counting records instead of exporting them.
func CountRecords(ctx context.Context, plan Plan, keychainConfig KeychainConfig) (Summary, error) {
	keychain := newLoginKeychain(keychainConfig)
	return runner{writer: discard{}, extractors: extractorsWith(keychain), keys: keysFromFile(nil)}.run(ctx, plan)
}

type discard struct{}

func (discard) Write(category.Category, []record.Record) error { return nil }
