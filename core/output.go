package core

import (
	"bytes"
	"encoding/json"
	"hack-browser-data/log"
	"hack-browser-data/utils"
	"os"

	"github.com/gocarina/gocsv"
)

func (b BrowserData) OutPutCsv(dir, format string) error {
	switch {
	case len(b.BookmarkSlice) != 0:
		filename := utils.FormatFileName(dir, utils.Bookmarks, format)
		file, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC|os.O_APPEND, 0644)
		defer file.Close()
		if err != nil {
			log.Errorf("create file %s fail", filename)
		}
		err = gocsv.MarshalFile(b.BookmarkSlice, file)
		if err != nil {
			log.Error(err)
		}
		fallthrough
	case len(b.LoginDataSlice) != 0:
		filename := utils.FormatFileName(dir, utils.LoginData, format)
		file, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC|os.O_APPEND, 0644)
		defer file.Close()
		if err != nil {
			log.Errorf("create file %s fail", filename)
		}
		err = gocsv.MarshalFile(b.LoginDataSlice, file)
		if err != nil {
			log.Error(err)
		}
		fallthrough
	case len(b.CookieMap) != 0:
		filename := utils.FormatFileName(dir, utils.Cookies, format)
		file, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC|os.O_APPEND, 0644)
		defer file.Close()
		if err != nil {
			log.Errorf("create file %s fail", filename)
		}
		var tempSlice []cookies
		for _, v := range b.CookieMap {
			tempSlice = append(tempSlice, v...)
		}
		err = gocsv.MarshalFile(tempSlice, file)
		if err != nil {
			log.Error(err)
		}
		fallthrough
	case len(b.HistorySlice) != 0:
		filename := utils.FormatFileName(dir, utils.History, format)
		file, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC|os.O_APPEND, 0644)
		defer file.Close()
		if err != nil {
			log.Errorf("create file %s fail", filename)
		}
		err = gocsv.MarshalFile(b.HistorySlice, file)
		if err != nil {
			log.Error(err)
		}
	}
	return nil
}

func (b BrowserData) OutPutJson(dir, format string) error {
	switch {
	case len(b.BookmarkSlice) != 0:
		filename := utils.FormatFileName(dir, utils.Bookmarks, format)
		file, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC|os.O_APPEND, 0644)
		defer file.Close()
		if err != nil {
			log.Errorf("create file %s fail", filename)
		}
		w := new(bytes.Buffer)
		enc := json.NewEncoder(w)
		enc.SetEscapeHTML(false)
		enc.SetIndent("", "\t")
		enc.Encode(b.BookmarkSlice)
		file.Write(w.Bytes())
		fallthrough
	case len(b.CookieMap) != 0:
		filename := utils.FormatFileName(dir, utils.Cookies, format)
		file, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC|os.O_APPEND, 0644)
		defer file.Close()
		if err != nil {
			log.Errorf("create file %s fail", filename)
		}
		w := new(bytes.Buffer)
		enc := json.NewEncoder(w)
		enc.SetEscapeHTML(false)
		enc.SetIndent("", "\t")
		err = enc.Encode(b.CookieMap)
		if err != nil {
			log.Println(err)
		}
		file.Write(w.Bytes())
		fallthrough
	case len(b.HistorySlice) != 0:
		filename := utils.FormatFileName(dir, utils.History, format)
		file, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC|os.O_APPEND, 0644)
		defer file.Close()
		if err != nil {
			log.Errorf("create file %s fail", filename)
		}
		w := new(bytes.Buffer)
		enc := json.NewEncoder(w)
		enc.SetEscapeHTML(false)
		enc.SetIndent("", "\t")
		err = enc.Encode(b.HistorySlice)
		if err != nil {
			log.Println(err)
		}
		file.Write(w.Bytes())
		fallthrough
	case len(b.LoginDataSlice) != 0:
		filename := utils.FormatFileName(dir, utils.LoginData, format)
		file, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC|os.O_APPEND, 0644)
		defer file.Close()
		if err != nil {
			log.Errorf("create file %s fail", filename)
		}
		w := new(bytes.Buffer)
		enc := json.NewEncoder(w)
		enc.SetEscapeHTML(false)
		enc.SetIndent("", "\t")
		err = enc.Encode(b.LoginDataSlice)
		if err != nil {
			log.Println(err)
		}
		file.Write(w.Bytes())
	}
	return nil
}
