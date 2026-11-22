package app

import (
	"context"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/category"
	"github.com/moond4rk/hackbrowserdata/internal/record"
)

// List finds every browser of this OS. With detail it also counts each profile's records, running every category with
// no key and the keychain locked, so counting never prompts.
func List(ctx context.Context, detail bool) (Discovery, Summary, error) {
	home, err := homeDir()
	if err != nil {
		return Discovery{}, Summary{}, err
	}
	discovery := discover(ctx, browser.Supported(), browser.DiscoverOptions{HomeDir: home})
	if !detail || len(discovery.Dirs) == 0 {
		return discovery, Summary{}, nil
	}
	keychain := newLoginKeychain(loginKeychainPath(home), nil)
	counter := runner{writer: discard{}, extractors: extractorsFor, retrieveKeys: keysFromFile(nil), loginKeychain: keychain}
	summary := counter.run(ctx, discovery, category.All())
	if summary.Err != nil {
		return Discovery{}, Summary{}, summary.Err
	}
	return discovery, summary, nil
}

type discard struct{}

func (discard) Write(category.Category, []record.Record) error { return nil }
