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
	localState          = browser.FileSpec{Paths: []string{"Local State"}, Shared: true, Optional: true}
	loginData           = browser.FileSpec{Paths: []string{"Login Data"}, Optional: true}
	loginDataForAccount = browser.FileSpec{Paths: []string{accountLoginData}, Optional: true}
)

var defaultExtractors = browser.Extractors{
	category.Password:       newDecryptingExtractor(extractPasswords, loginData, loginDataForAccount),
	category.Cookie:         newDecryptingExtractor(extractCookies, newFileSpec("Network/Cookies", "Cookies")),
	category.Bookmark:       newPlainExtractor(extractBookmarks, newFileSpec("Bookmarks")),
	category.History:        newPlainExtractor(extractHistories, newFileSpec("History")),
	category.Download:       newPlainExtractor(extractDownloads, newFileSpec("History")),
	category.CreditCard:     newDecryptingExtractor(extractCreditCards, newFileSpec("Web Data")),
	category.Extension:      newPlainExtractor(extractExtensions, newFileSpec("Secure Preferences")),
	category.LocalStorage:   newPlainExtractor(extractLocalStorage, newDirectorySpec("Local Storage/leveldb")),
	category.SessionStorage: newPlainExtractor(extractSessionStorage, newDirectorySpec("Session Storage")),
}

func Extractors(dir browser.UserDataDir, _ browser.Profile) browser.Extractors {
	extractors := make(browser.Extractors, len(defaultExtractors))
	for dataCategory, extractor := range defaultExtractors {
		extractors[dataCategory] = extractor
	}
	if fork, ok := overrideFor(dir.Browser.Name); ok {
		for dataCategory, extractor := range fork.extractors {
			extractors[dataCategory] = extractor
		}
	}
	return extractors
}

func newFileSpec(paths ...string) browser.FileSpec { return browser.FileSpec{Paths: paths} }
func newDirectorySpec(path string) browser.FileSpec {
	return browser.FileSpec{Paths: []string{path}, IsDir: true}
}

// withKeyFiles adds the files the dir's master keys come from, so a decrypting extraction finds them in the snapshot.
func withKeyFiles(fileSpecs ...browser.FileSpec) []browser.FileSpec {
	return append(fileSpecs, KeyFileSpecs()...)
}

// newDecryptingExtractor parses each file in turn, so a store that is missing or broken keeps the others' records.
func newDecryptingExtractor(parse func(masterKeysFunc, string, browser.Collector) error, fileSpecs ...browser.FileSpec) browser.Extractor {
	return browser.Extractor{
		FileSpecs: withKeyFiles(fileSpecs...),
		Extract: func(ctx context.Context, profileSnapshot browser.ProfileSnapshot, out browser.Collector) error {
			var errs []error
			for _, fileSpec := range fileSpecs {
				path, err := profileSnapshot.Path(fileSpec)
				if fileSpec.Optional && errors.Is(err, fs.ErrNotExist) {
					continue
				}
				if err == nil {
					err = parse(masterKeysOf(ctx, profileSnapshot), path, out)
				}
				errs = append(errs, err)
			}
			return errors.Join(errs...)
		},
	}
}

func masterKeysOf(ctx context.Context, profileSnapshot browser.ProfileSnapshot) masterKeysFunc {
	return func() masterkey.Set { return profileSnapshot.MasterKeys(ctx) }
}

func newPlainExtractor(parse func(string, browser.Collector) error, fileSpec browser.FileSpec) browser.Extractor {
	return browser.Extractor{
		FileSpecs: []browser.FileSpec{fileSpec},
		Extract: func(_ context.Context, profileSnapshot browser.ProfileSnapshot, out browser.Collector) error {
			path, err := profileSnapshot.Path(fileSpec)
			if err != nil {
				return err
			}
			return parse(path, out)
		},
	}
}
