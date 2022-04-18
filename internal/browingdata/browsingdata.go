package browingdata

import (
	"path"

	"hack-browser-data/internal/browingdata/bookmark"
	"hack-browser-data/internal/browingdata/cookie"
	"hack-browser-data/internal/browingdata/creditcard"
	"hack-browser-data/internal/browingdata/download"
	"hack-browser-data/internal/browingdata/history"
	"hack-browser-data/internal/browingdata/password"
	"hack-browser-data/internal/item"
	"hack-browser-data/internal/log"
	"hack-browser-data/internal/utils/fileutil"
)

type Data struct {
	Sources map[item.Item]Source
}

type Source interface {
	Parse(masterKey []byte) error

	Name() string
}

func New(sources []item.Item) *Data {
	bd := &Data{
		Sources: make(map[item.Item]Source),
	}
	bd.addSource(sources)
	return bd
}

func (d *Data) Recovery(masterKey []byte) error {

	for _, source := range d.Sources {
		if err := source.Parse(masterKey); err != nil {
			log.Error(err)
		}
	}
	return nil
}

func (d *Data) Output(dir, browserName, flag string) {
	output := NewOutPutter(flag)

	for _, source := range d.Sources {

		filename := fileutil.Filename(browserName, source.Name(), output.Ext())

		f, err := output.CreateFile(dir, filename)
		if err != nil {
			log.Error(err)
		}
		if err := output.Write(source, f); err != nil {
			log.Error(err)
		}
		log.Noticef("output to file %s success", path.Join(dir, filename))
	}
}

func (d *Data) addSource(Sources []item.Item) {
	for _, source := range Sources {
		switch source {
		case item.ChromiumPassword:
			d.Sources[source] = &password.ChromiumPassword{}
		case item.ChromiumCookie:
			d.Sources[source] = &cookie.ChromiumCookie{}
		case item.ChromiumBookmark:
			d.Sources[source] = &bookmark.ChromiumBookmark{}
		case item.ChromiumHistory:
			d.Sources[source] = &history.ChromiumHistory{}
		case item.ChromiumDownload:
			d.Sources[source] = &download.ChromiumDownload{}
		case item.ChromiumCreditCard:
			d.Sources[source] = &creditcard.ChromiumCreditCard{}
		case item.YandexPassword:
			d.Sources[source] = &password.YandexPassword{}
		case item.YandexCreditCard:
			d.Sources[source] = &creditcard.YandexCreditCard{}
		case item.FirefoxPassword:
			d.Sources[source] = &password.FirefoxPassword{}
		case item.FirefoxCookie:
			d.Sources[source] = &cookie.FirefoxCookie{}
		case item.FirefoxBookmark:
			d.Sources[source] = &bookmark.FirefoxBookmark{}
		case item.FirefoxHistory:
			d.Sources[source] = &history.FirefoxHistory{}
		case item.FirefoxDownload:
			d.Sources[source] = &download.FirefoxDownload{}
		}
	}
}
