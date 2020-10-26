package core

import (
	"errors"
	"strings"

	"hack-browser-data/core/common"
	"hack-browser-data/log"
	"hack-browser-data/utils"
)

const (
	chromeName    = "Chrome"
	edgeName      = "Microsoft Edge"
	firefoxName   = "Firefox"
	speed360Name  = "360speed"
	qqBrowserName = "qq"
	braveName     = "Brave"
)

type Browser interface {
	// InitSecretKey is init chrome secret key, firefox's key always empty
	InitSecretKey() error

	// GetName return browser name
	GetName() string

	// GetSecretKey return browser secret key
	GetSecretKey() []byte

	// GetAllItems return all of items (password|bookmark|cookie|history)
	GetAllItems() ([]common.Item, error)

	// GetItem return single one from password|bookmark|cookie|history
	GetItem(itemName string) (common.Item, error)
}

const (
	cookie   = "cookie"
	history  = "history"
	bookmark = "bookmark"
	password = "password"
)

var (
	errItemNotSupported    = errors.New(`item not supported, default is "all", choose from history|password|bookmark|cookie`)
	errBrowserNotSupported = errors.New("browser not supported")
	errChromeSecretIsEmpty = errors.New("chrome secret is empty")
	errDbusSecretIsEmpty   = errors.New("dbus secret key is empty")
)

var (
	chromiumItems = map[string]struct {
		mainFile string
		newItem  func(mainFile, subFile string) common.Item
	}{
		bookmark: {
			mainFile: common.ChromeBookmarkFile,
			newItem:  common.NewBookmarks,
		},
		cookie: {
			mainFile: common.ChromeCookieFile,
			newItem:  common.NewCookies,
		},
		history: {
			mainFile: common.ChromeHistoryFile,
			newItem:  common.NewHistoryData,
		},
		password: {
			mainFile: common.ChromePasswordFile,
			newItem:  common.NewCPasswords,
		},
	}
	firefoxItems = map[string]struct {
		mainFile string
		subFile  string
		newItem  func(mainFile, subFile string) common.Item
	}{
		bookmark: {
			mainFile: common.FirefoxDataFile,
			newItem:  common.NewBookmarks,
		},
		cookie: {
			mainFile: common.FirefoxCookieFile,
			newItem:  common.NewCookies,
		},
		history: {
			mainFile: common.FirefoxDataFile,
			newItem:  common.NewHistoryData,
		},
		password: {
			mainFile: common.FirefoxKey4File,
			subFile:  common.FirefoxLoginFile,
			newItem:  common.NewFPasswords,
		},
	}
)

type Chromium struct {
	name        string
	profilePath string
	keyPath     string
	storage     string // use for linux browser
	secretKey   []byte
}

func NewChromium(profile, key, name, storage string) (Browser, error) {
	return &Chromium{profilePath: profile, keyPath: key, name: name, storage: storage}, nil
}

func (c *Chromium) GetName() string {
	return c.name
}

func (c *Chromium) GetSecretKey() []byte {
	return c.secretKey
}

func (c *Chromium) GetAllItems() (Items []common.Item, err error) {
	var items []common.Item
	for item, choice := range chromiumItems {
		m, err := utils.GetItemPath(c.profilePath, choice.mainFile)
		if err != nil {
			log.Errorf("%s find %s file failed, ERR:%s", c.name, item, err)
			continue
		}
		i := choice.newItem(m, "")
		log.Debugf("%s find %s File Success", c.name, item)
		items = append(items, i)
	}
	return items, nil
}

func (c *Chromium) GetItem(itemName string) (common.Item, error) {
	itemName = strings.ToLower(itemName)
	if item, ok := chromiumItems[itemName]; ok {
		m, err := utils.GetItemPath(c.profilePath, item.mainFile)
		if err != nil {
			log.Errorf("%s find %s file failed, ERR:%s", c.name, item.mainFile, err)
		}
		i := item.newItem(m, "")
		return i, nil
	} else {
		return nil, errItemNotSupported
	}
}

type Firefox struct {
	name        string
	profilePath string
	keyPath     string
}

func NewFirefox(profile, key, name, storage string) (Browser, error) {
	return &Firefox{profilePath: profile, keyPath: key, name: name}, nil
}

func (f *Firefox) GetAllItems() ([]common.Item, error) {
	var items []common.Item
	for item, choice := range firefoxItems {
		var (
			sub, main string
			err       error
		)
		if choice.subFile != "" {
			sub, err = utils.GetItemPath(f.profilePath, choice.subFile)
			if err != nil {
				log.Errorf("%s find %s file failed, ERR:%s", f.name, item, err)
				continue
			}
		}
		main, err = utils.GetItemPath(f.profilePath, choice.mainFile)
		if err != nil {
			log.Errorf("%s find %s file failed, ERR:%s", f.name, item, err)
			continue
		}
		i := choice.newItem(main, sub)
		log.Debugf("%s find %s file success", f.name, item)
		items = append(items, i)
	}
	return items, nil
}

func (f *Firefox) GetItem(itemName string) (common.Item, error) {
	itemName = strings.ToLower(itemName)
	if item, ok := firefoxItems[itemName]; ok {
		var (
			sub, main string
			err       error
		)
		if item.subFile != "" {
			sub, err = utils.GetItemPath(f.profilePath, item.subFile)
			if err != nil {
				log.Errorf("%s find %s file failed, ERR:%s", f.name, item.subFile, err)
			}
		}
		main, err = utils.GetItemPath(f.profilePath, item.mainFile)
		if err != nil {
			log.Errorf("%s find %s file failed, ERR:%s", f.name, item.mainFile, err)
		}
		i := item.newItem(main, sub)
		log.Debugf("%s find %s file success", f.name, item.mainFile)
		return i, nil
	} else {
		return nil, errItemNotSupported
	}
}

func (f *Firefox) GetName() string {
	return f.name
}

func (f *Firefox) GetSecretKey() []byte {
	return nil
}

func (f *Firefox) InitSecretKey() error {
	return nil
}

func PickBrowser(name string) ([]Browser, error) {
	var browsers []Browser
	name = strings.ToLower(name)
	if name == "all" {
		for _, v := range browserList {
			b, err := v.New(v.ProfilePath, v.KeyPath, v.Name, v.Storage)
			if err != nil {
				log.Error(err)
			}
			browsers = append(browsers, b)
		}
		return browsers, nil
	} else if choice, ok := browserList[name]; ok {
		b, err := choice.New(choice.ProfilePath, choice.KeyPath, choice.Name, choice.Storage)
		browsers = append(browsers, b)
		return browsers, err
	}
	return nil, errBrowserNotSupported
}

func ListBrowser() []string {
	var l []string
	for k := range browserList {
		l = append(l, k)
	}
	return l
}

func ListItem() []string {
	var l []string
	for k := range chromiumItems {
		l = append(l, k)
	}
	return l
}
