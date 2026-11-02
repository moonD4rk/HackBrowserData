package chromium

import (
	"context"
	"errors"
	"io/fs"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/category"
	"github.com/moond4rk/hackbrowserdata/internal/masterkey"
	"github.com/moond4rk/hackbrowserdata/internal/record"
)

var (
	localState          = browser.FileSpec{Paths: []string{"Local State"}, Shared: true, Optional: true}
	loginData           = browser.FileSpec{Paths: []string{"Login Data"}, Optional: true}
	loginDataForAccount = browser.FileSpec{Paths: []string{"Login Data For Account"}, Optional: true}
)

var defaultExtractors = browser.Extractors{
	category.Password:       {FileSpecs: []browser.FileSpec{loginData, loginDataForAccount, localState}, Extract: extractPasswordStores},
	category.Cookie:         newDecryptingExtractor(newFileSpec("Network/Cookies", "Cookies"), extractCookies),
	category.Bookmark:       newPlainExtractor(newFileSpec("Bookmarks"), extractBookmarks),
	category.History:        newPlainExtractor(newFileSpec("History"), extractHistories),
	category.Download:       newPlainExtractor(newFileSpec("History"), extractDownloads),
	category.CreditCard:     newDecryptingExtractor(newFileSpec("Web Data"), extractCreditCards),
	category.Extension:      newPlainExtractor(newFileSpec("Secure Preferences"), extractExtensions),
	category.LocalStorage:   newPlainExtractor(newDirectorySpec("Local Storage/leveldb"), extractLocalStorage),
	category.SessionStorage: newPlainExtractor(newDirectorySpec("Session Storage"), extractSessionStorage),
}

// overrides is keyed by Browser.Name, never UserDataDirID, which differs for a second user data dir of one browser.
var overrides = map[string]browser.Extractors{
	"yandex": {
		category.Password:   newDecryptingExtractor(newFileSpec("Ya Passman Data"), extractYandexPasswords),
		category.CreditCard: newDecryptingExtractor(newFileSpec("Ya Credit Cards"), extractYandexCreditCards),
	},
	"opera":    operaExtractors,
	"opera-gx": operaExtractors,
	"vought":   operaExtractors,
}

var operaExtractors = browser.Extractors{
	category.Extension: newPlainExtractor(newFileSpec("Secure Preferences"), extractOperaExtensions),
}

func Extractors(dir browser.UserDataDir, _ browser.Profile) browser.Extractors {
	extractors := make(browser.Extractors, len(defaultExtractors))
	for dataCategory, extractor := range defaultExtractors {
		extractors[dataCategory] = extractor
	}
	for dataCategory, extractor := range overrides[dir.Browser.Name] {
		extractors[dataCategory] = extractor
	}
	return extractors
}

func newFileSpec(paths ...string) browser.FileSpec { return browser.FileSpec{Paths: paths} }
func newDirectorySpec(path string) browser.FileSpec {
	return browser.FileSpec{Paths: []string{path}, IsDir: true}
}

// newDecryptingExtractor lists Local State beside fileSpec, so the keys that decrypt it are in the snapshot.
func newDecryptingExtractor(fileSpec browser.FileSpec, parse func(masterkey.Set, string, browser.Collector) error) browser.Extractor {
	return browser.Extractor{
		FileSpecs: []browser.FileSpec{fileSpec, localState},
		Extract: func(ctx context.Context, profileSnapshot browser.ProfileSnapshot, out browser.Collector) error {
			path, err := profileSnapshot.Path(fileSpec)
			if err != nil {
				return err
			}
			return parse(profileSnapshot.MasterKeys(ctx), path, out)
		},
	}
}

func newPlainExtractor(fileSpec browser.FileSpec, parse func(string, browser.Collector) error) browser.Extractor {
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

// extractPasswordStores reads both stores: a synced-only profile has one, and one failing keeps the other's logins.
func extractPasswordStores(ctx context.Context, profileSnapshot browser.ProfileSnapshot, out browser.Collector) error {
	stores := []struct {
		fileSpec browser.FileSpec
		store    record.Store
	}{
		{loginData, record.StoreLocal},
		{loginDataForAccount, record.StoreAccount},
	}
	var errs []error
	for _, loginStore := range stores {
		path, err := profileSnapshot.Path(loginStore.fileSpec)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err == nil {
			err = extractPasswords(profileSnapshot.MasterKeys(ctx), path, loginStore.store, out)
		}
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}
