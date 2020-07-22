package core

import (
	"errors"
	"hack-browser-data/core/common"
	"hack-browser-data/log"
	"hack-browser-data/utils"
	"path/filepath"
	"strings"
)

const (
	chromeName    = "Chrome"
	edgeName      = "Microsoft Edge"
	firefoxName   = "Firefox"
	speed360Name  = "360speed"
	qqBrowserName = "qq"
)

type Browser interface {
	GetProfilePath(filename string) (err error)
	InitSecretKey() error
	ParseDB()
	OutPut(dir, format string)
}

type chromium struct {
	ProfilePath string
	KeyPath     string
	Name        string
	SecretKey   []byte
	FileLists   []FileList
	Data        common.BrowserData
}

type firefox struct {
	ProfilePath string
	KeyPath     string
	Name        string
	FileLists   []FileList
	Data        common.BrowserData
}

type FileList struct {
	name     string
	mainFile string
	mainPath string
	subFile  string
	subPath  string
}

const (
	cookie   = "cookie"
	history  = "history"
	bookmark = "bookmark"
	password = "password"
)

var (
	ErrDataNotSupported    = errors.New(`not supported, default is "all", choose from history|password|bookmark|cookie`)
	ErrBrowserNotSupported = errors.New("browser not supported")
	chromiumParseList      = map[string]FileList{
		cookie: {
			name:     cookie,
			mainFile: common.ChromeCookies,
		},
		history: {
			name:     history,
			mainFile: common.ChromeHistory,
		},
		bookmark: {
			name:     bookmark,
			mainFile: common.ChromeBookmarks,
		},
		password: {
			name:     password,
			mainFile: common.ChromePassword,
		},
	}
	firefoxParseList = map[string]FileList{
		cookie: {
			name:     cookie,
			mainFile: common.FirefoxCookie,
		},
		history: {
			name:     history,
			mainFile: common.FirefoxData,
		},
		bookmark: {
			name:     bookmark,
			mainFile: common.FirefoxData,
		},
		password: {
			name:     password,
			mainFile: common.FirefoxKey4DB,
			subFile:  common.FirefoxLoginData,
		},
	}
)

func (c *chromium) GetProfilePath(filename string) (err error) {
	filename = strings.ToLower(filename)
	if filename == "all" {
		for _, v := range chromiumParseList {
			m, err := filepath.Glob(c.ProfilePath + v.mainFile)
			if err != nil {
				log.Error(err)
				continue
			}
			if len(m) > 0 {
				log.Debugf("%s find %s File Success", c.Name, v.name)
				log.Debugf("%s file location is %s", v, m[0])
				v.mainPath = m[0]
				c.FileLists = append(c.FileLists, v)
			} else {
				log.Errorf("%+v find %s failed", c.Name, v.name)
			}
		}
	} else if v, ok := chromiumParseList[filename]; ok {
		m, err := filepath.Glob(c.ProfilePath + v.mainFile)
		if err != nil {
			log.Error(err)
		}
		if len(m) > 0 {
			log.Debugf("%s find %s File Success", c.Name, v)
			log.Debugf("%s file location is %s", v, m[0])
			v.mainPath = m[0]
			c.FileLists = append(c.FileLists, v)
		}
	} else {
		return ErrDataNotSupported
	}
	return nil
}

func (c *chromium) ParseDB() {
	for _, v := range c.FileLists {
		err := utils.CopyDB(v.mainPath, filepath.Base(v.mainPath))
		if err != nil {
			log.Error(err)
		}
		switch v.name {
		case bookmark:
			if err := chromeParse(c.SecretKey, &c.Data.Bookmarks); err != nil {
				log.Error(err)
			}
		case history:
			if err := chromeParse(c.SecretKey, &c.Data.History); err != nil {
				log.Error(err)
			}
		case password:
			if err := chromeParse(c.SecretKey, &c.Data.Logins); err != nil {
				log.Error(err)
			}
		case cookie:
			if err := chromeParse(c.SecretKey, &c.Data.Cookies); err != nil {
				log.Error(err)
			}
		}
	}
}

func (c *chromium) OutPut(dir, format string) {
	c.Data.Sorted()
	switch format {
	case "json":
		for _, v := range c.FileLists {
			switch v.name {
			case bookmark:
				if err := outPutJson(c.Name, dir, &c.Data.Bookmarks); err != nil {
					log.Error(err)
				}
			case history:
				if err := outPutJson(c.Name, dir, &c.Data.History); err != nil {
					log.Error(err)
				}
			case password:
				if err := outPutJson(c.Name, dir, &c.Data.Logins); err != nil {
					log.Error(err)
				}
			case cookie:
				if err := outPutJson(c.Name, dir, &c.Data.Cookies); err != nil {
					log.Error(err)
				}
			}
		}
	case "csv":
		for _, v := range c.FileLists {
			switch v.name {
			case bookmark:
				if err := outPutCsv(c.Name, dir, &c.Data.Bookmarks); err != nil {
					log.Error(err)
				}
			case history:
				if err := outPutCsv(c.Name, dir, &c.Data.History); err != nil {
					log.Error(err)
				}
			case password:
				if err := outPutCsv(c.Name, dir, &c.Data.Logins); err != nil {
					log.Error(err)
				}
			case cookie:
				if err := outPutCsv(c.Name, dir, &c.Data.Cookies); err != nil {
					log.Error(err)
				}
			}
		}
	}
}

func decryptChromium(profile, key, name string) (Browser, error) {
	return &chromium{ProfilePath: profile, KeyPath: key, Name: name}, nil
}

func (f *firefox) ParseDB() {
	for _, v := range f.FileLists {
		err := utils.CopyDB(v.mainPath, filepath.Base(v.mainPath))
		if v.subPath != "" {
			err := utils.CopyDB(v.subPath, filepath.Base(v.subPath))
			if err != nil {
				log.Error(err)
			}
		}
		if err != nil {
			log.Error(err)
		}
		switch v.name {
		case password:
			if err := firefoxParse(&f.Data.Logins); err != nil {
				log.Error(err)
			}
		case bookmark:
			if err := firefoxParse(&f.Data.Bookmarks); err != nil {
				log.Error(err)
			}
		case history:
			if err := firefoxParse(&f.Data.History); err != nil {
				log.Error(err)
			}
		case cookie:
			if err := firefoxParse(&f.Data.Cookies); err != nil {
				log.Error(err)
			}
		}
	}
}

func (f *firefox) GetProfilePath(filename string) (err error) {
	filename = strings.ToLower(filename)
	if filename == "all" {
		for _, v := range firefoxParseList {
			m, err := filepath.Glob(f.ProfilePath + v.mainFile)
			if v.subFile != "" {
				s, err := filepath.Glob(f.ProfilePath + v.subFile)
				if err != nil {
					log.Error(err)
					continue
				}
				if len(s) > 0 {
					log.Debugf("%s find %s File Success", f.Name, v.name)
					log.Debugf("%s file location is %s", v, s[0])
					v.subPath = s[0]
				}
			}
			if err != nil {
				log.Error(err)
				continue
			}
			if len(m) > 0 {
				log.Debugf("%s find %s File Success", f.Name, v.name)
				log.Debugf("%+v file location is %s", v, m[0])
				v.mainPath = m[0]
				f.FileLists = append(f.FileLists, v)
			} else {
				log.Errorf("%s find %s failed", f.Name, v.name)
			}
		}
	} else if v, ok := firefoxParseList[filename]; ok {
		m, err := filepath.Glob(f.ProfilePath + v.mainFile)
		if err != nil {
			log.Error(err)
		}
		if len(m) > 0 {
			log.Debugf("%s find %s File Success", f.Name, v)
			log.Debugf("%s file location is %s", v, m[0])
			v.mainPath = m[0]
			f.FileLists = append(f.FileLists, v)
		}
	} else {
		return ErrDataNotSupported
	}
	return nil
}

func (f *firefox) OutPut(dir, format string) {
	f.Data.Sorted()
	switch format {
	case "json":
		for _, v := range f.FileLists {
			switch v.name {
			case bookmark:
				if err := outPutJson(f.Name, dir, &f.Data.Bookmarks); err != nil {
					log.Error(err)
				}
			case history:
				if err := outPutJson(f.Name, dir, &f.Data.History); err != nil {
					log.Error(err)
				}
			case password:
				if err := outPutJson(f.Name, dir, &f.Data.Logins); err != nil {
					log.Error(err)
				}
			case cookie:
				if err := outPutJson(f.Name, dir, &f.Data.Cookies); err != nil {
					log.Error(err)
				}
			}
		}
	case "csv":
		for _, v := range f.FileLists {
			switch v.name {
			case bookmark:
				if err := outPutCsv(f.Name, dir, &f.Data.Bookmarks); err != nil {
					log.Error(err)
				}
			case history:
				if err := outPutCsv(f.Name, dir, &f.Data.History); err != nil {
					log.Error(err)
				}
			case password:
				if err := outPutCsv(f.Name, dir, &f.Data.Logins); err != nil {
					log.Error(err)
				}
			case cookie:
				if err := outPutCsv(f.Name, dir, &f.Data.Cookies); err != nil {
					log.Error(err)
				}
			}
		}
	}
}

func (f *firefox) InitSecretKey() error {
	return nil
}

func decryptFirefox(profile, key, name string) (Browser, error) {
	return &firefox{ProfilePath: profile, KeyPath: key, Name: name}, nil
}

func PickBrowsers(name string) ([]Browser, error) {
	var browsers []Browser
	name = strings.ToLower(name)
	if name == "all" {
		for _, v := range browserList {
			b, err := v.New(v.ProfilePath, v.KeyPath, v.Name)
			if err != nil {
				log.Error(err)
			}
			browsers = append(browsers, b)
		}
		return browsers, nil
	} else if choice, ok := browserList[name]; ok {
		b, err := choice.New(choice.ProfilePath, choice.KeyPath, choice.Name)
		browsers = append(browsers, b)
		return browsers, err
	}
	return nil, ErrBrowserNotSupported
}

func chromeParse(key []byte, f common.Formatter) error {
	return f.ChromeParse(key)
}

func firefoxParse(f common.Formatter) error {
	return f.FirefoxParse()
}

func outPutJson(name, dir string, f common.Formatter) error {
	return f.OutPutJson(name, dir)
}

func outPutCsv(name, dir string, f common.Formatter) error {
	return f.OutPutCsv(name, dir)
}

func ListBrowser() []string {
	var l []string
	for k := range browserList {
		l = append(l, k)
	}
	return l
}
