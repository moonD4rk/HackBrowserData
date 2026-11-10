package chromium

import (
	"context"
	"errors"
	"io/fs"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/category"
	"github.com/moond4rk/hackbrowserdata/internal/masterkey"
)

var (
	localState          = browser.Source{Path: "Local State", Shared: true, Optional: true}
	loginData           = browser.Source{Path: "Login Data", Optional: true}
	loginDataForAccount = browser.Source{Path: accountLoginData, Optional: true}
	cookies             = browser.Source{Path: "Network/Cookies", Fallbacks: []string{"Cookies"}}
	bookmarks           = browser.Source{Path: "Bookmarks"}
	history             = browser.Source{Path: "History"}
	webData             = browser.Source{Path: "Web Data"}
	securePreferences   = browser.Source{Path: "Secure Preferences"}
	localStorage        = browser.Source{Path: "Local Storage/leveldb", IsDir: true}
	sessionStorage      = browser.Source{Path: "Session Storage", IsDir: true}
	yaPassmanData       = browser.Source{Path: "Ya Passman Data"}
	yaCreditCards       = browser.Source{Path: "Ya Credit Cards"}
)

var defaultExtractors = browser.Extractors{
	category.Password:       newDecryptingExtractor(extractPasswords, loginData, loginDataForAccount),
	category.Cookie:         newDecryptingExtractor(extractCookies, cookies),
	category.Bookmark:       newPlainExtractor(extractBookmarks, bookmarks),
	category.History:        newPlainExtractor(extractHistories, history),
	category.Download:       newPlainExtractor(extractDownloads, history),
	category.CreditCard:     newDecryptingExtractor(extractCreditCards, webData),
	category.Extension:      newPlainExtractor(extractExtensions, securePreferences),
	category.LocalStorage:   newPlainExtractor(extractLocalStorage, localStorage),
	category.SessionStorage: newPlainExtractor(extractSessionStorage, sessionStorage),
}

func Extractors(dir browser.UserDataDir, _ browser.Profile) browser.Extractors {
	return kindExtractors(vaultKindOf(dir))
}

func kindExtractors(vaultKind string) browser.Extractors {
	extractors := make(browser.Extractors, len(defaultExtractors))
	for dataCategory, extractor := range defaultExtractors {
		extractors[dataCategory] = extractor
	}
	if fork, ok := overrideForVaultKind(vaultKind); ok {
		for dataCategory, extractor := range fork.extractors {
			extractors[dataCategory] = extractor
		}
	}
	return extractors
}

// newDecryptingExtractor parses each file in turn, so a store that is missing or broken keeps the others' records.
func newDecryptingExtractor(parse func(masterKeysFunc, string, browser.Collector) error, sources ...browser.Source) browser.Extractor {
	return browser.Extractor{
		// The dir's master keys come from KeySources, so a decrypting extraction must find them in the snapshot too.
		Sources: append(sources, KeySources()...),
		Extract: func(ctx context.Context, input browser.Input, out browser.Collector) error {
			var errs []error
			for _, source := range sources {
				path, err := input.Path(source)
				if source.Optional && errors.Is(err, fs.ErrNotExist) {
					continue
				}
				if err == nil {
					err = parse(masterKeysOf(ctx, input), path, out)
				}
				errs = append(errs, err)
			}
			return errors.Join(errs...)
		},
	}
}

func masterKeysOf(ctx context.Context, input browser.Input) masterKeysFunc {
	return func() masterkey.Set { return input.MasterKeys(ctx) }
}

func newPlainExtractor(parse func(string, browser.Collector) error, source browser.Source) browser.Extractor {
	return browser.Extractor{
		Sources: []browser.Source{source},
		Extract: func(_ context.Context, input browser.Input, out browser.Collector) error {
			path, err := input.Path(source)
			if err != nil {
				return err
			}
			return parse(path, out)
		},
	}
}
