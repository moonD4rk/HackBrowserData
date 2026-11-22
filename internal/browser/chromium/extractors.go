package chromium

import (
	"context"
	"errors"
	"fmt"
	"io/fs"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/category"
	"github.com/moond4rk/hackbrowserdata/internal/masterkey"
)

var (
	localState          = browser.Source{RelativePath: "Local State", FromUserDataDir: true, Shared: true, Optional: true}
	loginData           = browser.Source{RelativePath: "Login Data", Optional: true}
	loginDataForAccount = browser.Source{RelativePath: accountLoginData, Optional: true}
	cookies             = browser.Source{RelativePath: "Network/Cookies", Fallbacks: []string{"Cookies"}}
	bookmarks           = browser.Source{RelativePath: "Bookmarks"}
	history             = browser.Source{RelativePath: "History"}
	webData             = browser.Source{RelativePath: "Web Data"}
	securePreferences   = browser.Source{RelativePath: "Secure Preferences"}
	preferences         = browser.Source{RelativePath: "Preferences", Optional: true}
	localStorage        = browser.Source{RelativePath: "Local Storage/leveldb", IsDir: true}
	sessionStorage      = browser.Source{RelativePath: "Session Storage", IsDir: true}
	yaPassmanData       = browser.Source{RelativePath: "Ya Passman Data"}
	yaCreditCards       = browser.Source{RelativePath: "Ya Credit Cards"}
)

// extractor is how one category is read: the files it needs, and how it parses them with the dir's master keys.
type extractor struct {
	sources         []browser.Source
	needsMasterKeys bool
	extract         func(captured browser.ProfileSnapshot, masterKeys masterkey.Set, out browser.Collector) error
}

var defaultExtractors = map[category.Category]extractor{
	category.Password:       newDecryptingExtractor(extractPasswords, loginData, loginDataForAccount),
	category.Cookie:         newDecryptingExtractor(extractCookies, cookies),
	category.Bookmark:       newPlainExtractor(extractBookmarks, bookmarks),
	category.History:        newPlainExtractor(extractHistories, history),
	category.Download:       newPlainExtractor(extractDownloads, history),
	category.CreditCard:     newDecryptingExtractor(extractCreditCards, webData),
	category.Extension:      newExtensionExtractor(extensionSettingsKeys...),
	category.LocalStorage:   newPlainExtractor(extractLocalStorage, localStorage),
	category.SessionStorage: newPlainExtractor(extractSessionStorage, sessionStorage),
}

// Sources lists the files of each category the dir's browser supports; every profile of a dir reads the same ones.
func Sources(dir browser.UserDataDir, _ browser.Profile) map[category.Category][]browser.Source {
	extractors := extractorsFor(dir)
	sources := make(map[category.Category][]browser.Source, len(extractors))
	for dataCategory, extractor := range extractors {
		sources[dataCategory] = extractor.sources
	}
	return sources
}

// Extract decrypts with masterKeys, which lack each key version that could not be retrieved.
func Extract(
	_ context.Context, captured browser.ProfileSnapshot, masterKeys masterkey.Set, dataCategory category.Category, out browser.Collector,
) error {
	extractor, ok := extractorsFor(captured.UserDataDir)[dataCategory]
	if !ok {
		return fmt.Errorf("%s has no %s extractor", captured.UserDataDir.Browser.Name, dataCategory)
	}
	return extractor.extract(captured, masterKeys, out)
}

// NeedsMasterKeys reports whether dir seals dataCategory's values with its master keys, so a run that asks only for the
// other categories never retrieves a key.
func NeedsMasterKeys(dir browser.UserDataDir, dataCategory category.Category) bool {
	return extractorsFor(dir)[dataCategory].needsMasterKeys
}

func extractorsFor(dir browser.UserDataDir) map[category.Category]extractor {
	extractors := make(map[category.Category]extractor, len(defaultExtractors))
	for dataCategory, extractor := range defaultExtractors {
		extractors[dataCategory] = extractor
	}
	if fork, ok := overrideForDir(dir); ok {
		for dataCategory, extractor := range fork.extractors {
			extractors[dataCategory] = extractor
		}
		for _, dataCategory := range fork.unsupported {
			delete(extractors, dataCategory)
		}
	}
	return extractors
}

// newDecryptingExtractor parses each file in turn, so a store that is missing or broken keeps the others' records.
func newDecryptingExtractor(parse func(masterkey.Set, string, browser.Collector) error, sources ...browser.Source) extractor {
	return extractor{
		// The dir's master keys come from KeySources, so a decrypting extraction must find them in the snapshot too.
		sources:         append(sources, KeySources()...),
		needsMasterKeys: true,
		extract: func(captured browser.ProfileSnapshot, masterKeys masterkey.Set, out browser.Collector) error {
			var errs []error
			for _, source := range sources {
				path, err := captured.Path(source)
				if source.Optional && errors.Is(err, fs.ErrNotExist) {
					continue
				}
				if err == nil {
					err = parse(masterKeys, path, out)
				}
				errs = append(errs, err)
			}
			return errors.Join(errs...)
		},
	}
}

func newPlainExtractor(parse func(string, browser.Collector) error, source browser.Source) extractor {
	return extractor{
		sources: []browser.Source{source},
		extract: func(captured browser.ProfileSnapshot, _ masterkey.Set, out browser.Collector) error {
			path, err := captured.Path(source)
			if err != nil {
				return err
			}
			return parse(path, out)
		},
	}
}
