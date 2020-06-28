package core

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"hack-browser-data/log"
	"hack-browser-data/utils"
	"io"
	"os"

	"github.com/gocarina/gocsv"
)

func (b BrowserData) OutPutCsv(dir, browser, format string) error {
	switch {
	case len(b.BookmarkSlice) != 0:
		filename := utils.FormatFileName(dir, browser, utils.Bookmarks, format)
		file, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC|os.O_APPEND, 0644)
		defer file.Close()
		if err != nil {
			log.Errorf("create file %s fail", filename)
		}
		gocsv.SetCSVWriter(func(out io.Writer) *gocsv.SafeCSVWriter {
			writer := csv.NewWriter(out)
			writer.Comma = '	'
			return gocsv.NewSafeCSVWriter(writer)
		})
		err = gocsv.MarshalFile(b.BookmarkSlice, file)
		if err != nil {
			log.Error(err)
		}
		fmt.Printf("%s Get %d bookmarks, filename is %s \n", log.Prefix, len(b.BookmarkSlice), filename)
		fallthrough
	case len(b.LoginDataSlice) != 0:
		filename := utils.FormatFileName(dir, browser, utils.LoginData, format)
		file, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC|os.O_APPEND, 0644)
		defer file.Close()
		if err != nil {
			log.Errorf("create file %s fail", filename)
		}
		err = gocsv.MarshalFile(b.LoginDataSlice, file)
		if err != nil {
			log.Error(err)
		}
		fmt.Printf("%s Get %d login data, filename is %s \n", log.Prefix, len(b.LoginDataSlice), filename)
		fallthrough
	case len(b.CookieMap) != 0:
		filename := utils.FormatFileName(dir, browser, utils.Cookies, format)
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
		fmt.Printf("%s Get %d cookies, filename is %s \n", log.Prefix, len(b.CookieMap), filename)
		fallthrough
	case len(b.HistorySlice) != 0:
		filename := utils.FormatFileName(dir, browser, utils.History, format)
		file, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC|os.O_APPEND, 0644)
		defer file.Close()
		if err != nil {
			log.Errorf("create file %s fail", filename)
		}
		err = gocsv.MarshalFile(b.HistorySlice, file)
		if err != nil {
			log.Error(err)
		}
		fmt.Printf("%s Get %d login data, filename is %s \n", log.Prefix, len(b.HistorySlice), filename)
	}
	return nil
}

func (b BrowserData) OutPutJson(dir, browser, format string) error {
	switch {
	case len(b.BookmarkSlice) != 0:
		filename := utils.FormatFileName(dir, browser, utils.Bookmarks, format)
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
		fmt.Printf("%s Get %d bookmarks, filename is %s \n", log.Prefix, len(b.BookmarkSlice), filename)
		fallthrough
	case len(b.CookieMap) != 0:
		filename := utils.FormatFileName(dir, browser, utils.Cookies, format)
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
		fmt.Printf("%s Get %d cookies, filename is %s \n", log.Prefix, len(b.CookieMap), filename)
		fallthrough
	case len(b.HistorySlice) != 0:
		filename := utils.FormatFileName(dir, browser, utils.History, format)
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
		fmt.Printf("%s Get %d history, filename is %s \n", log.Prefix, len(b.HistorySlice), filename)
		fallthrough
	case len(b.LoginDataSlice) != 0:
		filename := utils.FormatFileName(dir, browser, utils.LoginData, format)
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
		fmt.Printf("%s Get %d login data, filename is %s \n", log.Prefix, len(b.LoginDataSlice), filename)
	}
	return nil
}
