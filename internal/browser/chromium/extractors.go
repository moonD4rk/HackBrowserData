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
	localState          = browser.Source{Paths: []string{"Local State"}, Shared: true, Optional: true}
	loginData           = browser.Source{Paths: []string{"Login Data"}, Optional: true}
	loginDataForAccount = browser.Source{Paths: []string{"Login Data For Account"}, Optional: true}
)

var defaultExtractors = browser.Extractors{
	category.Password:       {Sources: []browser.Source{loginData, loginDataForAccount, localState}, Extract: extractPasswordStores},
	category.Cookie:         encrypted(file("Network/Cookies", "Cookies"), extractCookies),
	category.Bookmark:       plain(file("Bookmarks"), extractBookmarks),
	category.History:        plain(file("History"), extractHistories),
	category.Download:       plain(file("History"), extractDownloads),
	category.CreditCard:     encrypted(file("Web Data"), extractCreditCards),
	category.Extension:      plain(file("Secure Preferences"), extractExtensions),
	category.LocalStorage:   plain(directory("Local Storage/leveldb"), extractLocalStorage),
	category.SessionStorage: plain(directory("Session Storage"), extractSessionStorage),
}

// overrides holds the forks with their own on-disk format: Yandex's password and card databases, and the Opera family's
// extension settings. It is keyed by Def.ID, never by DirID, which differs for a second user data dir of one browser.
var overrides = map[string]browser.Extractors{
	"yandex": {
		category.Password:   encrypted(file("Ya Passman Data"), extractYandexPasswords),
		category.CreditCard: encrypted(file("Ya Credit Cards"), extractYandexCreditCards),
	},
	"opera":    operaExtractors,
	"opera-gx": operaExtractors,
	"vought":   operaExtractors,
}

var operaExtractors = browser.Extractors{
	category.Extension: plain(file("Secure Preferences"), extractOperaExtensions),
}

// Extractors maps every category a Chromium profile supports to its extractor, with the forks' own formats applied.
func Extractors(dir browser.UserDataDir, _ browser.Profile) browser.Extractors {
	ex := make(browser.Extractors, len(defaultExtractors))
	for c, e := range defaultExtractors {
		ex[c] = e
	}
	for c, e := range overrides[dir.Def.ID] {
		ex[c] = e
	}
	return ex
}

func file(paths ...string) browser.Source  { return browser.Source{Paths: paths} }
func directory(path string) browser.Source { return browser.Source{Paths: []string{path}, IsDir: true} }

// encrypted lists Local State beside src, so the keys that decrypt src can be retrieved from the snapshot.
func encrypted(src browser.Source, parse func(masterkey.Set, string, browser.Collector) error) browser.Extractor {
	return browser.Extractor{
		Sources: []browser.Source{src, localState},
		Extract: func(ctx context.Context, ps browser.ProfileSnapshot, out browser.Collector) error {
			path, err := ps.Path(src)
			if err != nil {
				return err
			}
			return parse(ps.MasterKeys(ctx), path, out)
		},
	}
}

func plain(src browser.Source, parse func(string, browser.Collector) error) browser.Extractor {
	return browser.Extractor{
		Sources: []browser.Source{src},
		Extract: func(_ context.Context, ps browser.ProfileSnapshot, out browser.Collector) error {
			path, err := ps.Path(src)
			if err != nil {
				return err
			}
			return parse(path, out)
		},
	}
}

// extractPasswordStores reads both password databases: a profile that only ever synced passwords has just the account
// one, and one database failing does not hide the other's logins.
func extractPasswordStores(ctx context.Context, ps browser.ProfileSnapshot, out browser.Collector) error {
	stores := []struct {
		src   browser.Source
		store record.Store
	}{
		{loginData, record.StoreLocal},
		{loginDataForAccount, record.StoreAccount},
	}
	var errs []error
	for _, s := range stores {
		path, err := ps.Path(s.src)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err == nil {
			err = extractPasswords(ps.MasterKeys(ctx), path, s.store, out)
		}
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// emit hands rows to out by pointer, because out stamps each record in place.
func emit[T any, P interface {
	*T
	record.Record
}](out browser.Collector, rows []T) {
	for i := range rows {
		out.Add(P(&rows[i]))
	}
}
