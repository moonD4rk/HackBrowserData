package browingdata

import (
	"path"

	"github.com/moond4rk/HackBrowserData/browingdata/bookmark"
	"github.com/moond4rk/HackBrowserData/browingdata/cookie"
	"github.com/moond4rk/HackBrowserData/browingdata/creditcard"
	"github.com/moond4rk/HackBrowserData/browingdata/download"
	"github.com/moond4rk/HackBrowserData/browingdata/extension"
	"github.com/moond4rk/HackBrowserData/browingdata/history"
	"github.com/moond4rk/HackBrowserData/browingdata/localstorage"
	"github.com/moond4rk/HackBrowserData/browingdata/password"
	"github.com/moond4rk/HackBrowserData/browingdata/sessionstorage"
	"github.com/moond4rk/HackBrowserData/item"
	"github.com/moond4rk/HackBrowserData/log"
	"github.com/moond4rk/HackBrowserData/utils/fileutil"
)

type Data struct {
	sources map[item.Item]Source
}

type Source interface {
	Parse(masterKey []byte) error

	Name() string

	Len() int
}

func New(items []item.Item) *Data {
	bd := &Data{
		sources: make(map[item.Item]Source),
	}
	bd.addSources(items)
	return bd
}

func (d *Data) Recovery(masterKey []byte) error {
	for _, source := range d.sources {
		if err := source.Parse(masterKey); err != nil {
			log.Errorf("parse %s error %s", source.Name(), err.Error())
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
			log.Errorf("create file %s error %s", filename, err.Error())
			continue
		}
		if err := output.Write(source, f); err != nil {
			log.Errorf("write to file %s error %s", filename, err.Error())
			continue
		}
		if err := f.Close(); err != nil {
			log.Errorf("close file %s error %s", filename, err.Error())
			continue
		}
		log.Noticef("output to file %s success", path.Join(dir, filename))
	}
}

func (d *Data) addSources(items []item.Item) {
	for _, source := range items {
		switch source {
		case item.ChromiumPassword:
			d.sources[source] = &password.ChromiumPassword{}
		case item.ChromiumCookie:
			d.sources[source] = &cookie.ChromiumCookie{}
		case item.ChromiumBookmark:
			d.sources[source] = &bookmark.ChromiumBookmark{}
		case item.ChromiumHistory:
			d.sources[source] = &history.ChromiumHistory{}
		case item.ChromiumDownload:
			d.sources[source] = &download.ChromiumDownload{}
		case item.ChromiumCreditCard:
			d.sources[source] = &creditcard.ChromiumCreditCard{}
		case item.ChromiumLocalStorage:
			d.sources[source] = &localstorage.ChromiumLocalStorage{}
		case item.ChromiumSessionStorage:
			d.sources[source] = &sessionstorage.ChromiumSessionStorage{}
		case item.ChromiumExtension:
			d.sources[source] = &extension.ChromiumExtension{}
		case item.YandexPassword:
			d.sources[source] = &password.YandexPassword{}
		case item.YandexCreditCard:
			d.sources[source] = &creditcard.YandexCreditCard{}
		case item.FirefoxPassword:
			d.sources[source] = &password.FirefoxPassword{}
		case item.FirefoxCookie:
			d.sources[source] = &cookie.FirefoxCookie{}
		case item.FirefoxBookmark:
			d.sources[source] = &bookmark.FirefoxBookmark{}
		case item.FirefoxHistory:
			d.sources[source] = &history.FirefoxHistory{}
		case item.FirefoxDownload:
			d.sources[source] = &download.FirefoxDownload{}
		case item.FirefoxLocalStorage:
			d.sources[source] = &localstorage.FirefoxLocalStorage{}
		case item.FirefoxExtension:
			d.sources[source] = &extension.FirefoxExtension{}
		}
	}
}
