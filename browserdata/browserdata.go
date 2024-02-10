package browserdata

import (
	"log/slog"

	"github.com/moond4rk/hackbrowserdata/browserdata/bookmark"
	"github.com/moond4rk/hackbrowserdata/browserdata/cookie"
	"github.com/moond4rk/hackbrowserdata/browserdata/creditcard"
	"github.com/moond4rk/hackbrowserdata/browserdata/download"
	"github.com/moond4rk/hackbrowserdata/browserdata/extension"
	"github.com/moond4rk/hackbrowserdata/browserdata/history"
	"github.com/moond4rk/hackbrowserdata/browserdata/localstorage"
	"github.com/moond4rk/hackbrowserdata/browserdata/password"
	"github.com/moond4rk/hackbrowserdata/browserdata/sessionstorage"
	"github.com/moond4rk/hackbrowserdata/types"
	"github.com/moond4rk/hackbrowserdata/utils/fileutil"
)

type Data struct {
	sources map[types.DataType]Source
}

type Source interface {
	Parse(masterKey []byte) error

	Name() string

	Len() int
}

func New(items []types.DataType) *Data {
	bd := &Data{
		sources: make(map[types.DataType]Source),
	}
	bd.addSources(items)
	return bd
}

func (d *Data) Recovery(masterKey []byte) error {
	for _, source := range d.sources {
		if err := source.Parse(masterKey); err != nil {
			slog.Error("parse error", "source_data", source.Name(), "err", err.Error())
			continue
		}
	}
	return nil
}

func (d *Data) Output(dir, browserName, flag string) {
	output := newOutPutter(flag)

	for _, source := range d.sources {
		if source.Len() == 0 {
			// if the length of the export data is 0, then it is not necessary to output
			continue
		}
		filename := fileutil.ItemName(browserName, source.Name(), output.Ext())

		f, err := output.CreateFile(dir, filename)
		if err != nil {
			slog.Error("create file error", "filename", filename, "err", err.Error())
			continue
		}
		if err := output.Write(source, f); err != nil {
			slog.Error("write to file error", "filename", filename, "err", err.Error())
			continue
		}
		if err := f.Close(); err != nil {
			slog.Error("close file error", "filename", filename, "err", err.Error())
			continue
		}
		slog.Warn("export success", "filename", filename)
	}
}

func (d *Data) addSources(items []types.DataType) {
	for _, source := range items {
		switch source {
		case types.ChromiumPassword:
			d.sources[source] = &password.ChromiumPassword{}
		case types.ChromiumCookie:
			d.sources[source] = &cookie.ChromiumCookie{}
		case types.ChromiumBookmark:
			d.sources[source] = &bookmark.ChromiumBookmark{}
		case types.ChromiumHistory:
			d.sources[source] = &history.ChromiumHistory{}
		case types.ChromiumDownload:
			d.sources[source] = &download.ChromiumDownload{}
		case types.ChromiumCreditCard:
			d.sources[source] = &creditcard.ChromiumCreditCard{}
		case types.ChromiumLocalStorage:
			d.sources[source] = &localstorage.ChromiumLocalStorage{}
		case types.ChromiumSessionStorage:
			d.sources[source] = &sessionstorage.ChromiumSessionStorage{}
		case types.ChromiumExtension:
			d.sources[source] = &extension.ChromiumExtension{}
		case types.YandexPassword:
			d.sources[source] = &password.YandexPassword{}
		case types.YandexCreditCard:
			d.sources[source] = &creditcard.YandexCreditCard{}
		case types.FirefoxPassword:
			d.sources[source] = &password.FirefoxPassword{}
		case types.FirefoxCookie:
			d.sources[source] = &cookie.FirefoxCookie{}
		case types.FirefoxBookmark:
			d.sources[source] = &bookmark.FirefoxBookmark{}
		case types.FirefoxHistory:
			d.sources[source] = &history.FirefoxHistory{}
		case types.FirefoxDownload:
			d.sources[source] = &download.FirefoxDownload{}
		case types.FirefoxLocalStorage:
			d.sources[source] = &localstorage.FirefoxLocalStorage{}
		case types.FirefoxExtension:
			d.sources[source] = &extension.FirefoxExtension{}
		}
	}
}
