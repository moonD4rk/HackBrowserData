package common

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"hack-browser-data/log"
	"hack-browser-data/utils"

	"github.com/jszwec/csvutil"
)

var (
	utf8Bom        = []byte{239, 187, 191}
	errWriteToFile = errors.New("write to file failed")
)

func (b *Bookmarks) OutPutJson(browser, dir string) error {
	filename := utils.FormatFileName(dir, browser, "bookmark", "json")
	err := writeToJson(filename, b.bookmarks)
	if err != nil {
		log.Error(errWriteToFile)
		return err
	}
	fmt.Printf("%s Get %d bookmarks, filename is %s \n", log.Prefix, len(b.bookmarks), filename)
	return nil
}

func (h *History) OutPutJson(browser, dir string) error {
	filename := utils.FormatFileName(dir, browser, "history", "json")
	err := writeToJson(filename, h.history)
	if err != nil {
		log.Error(errWriteToFile)
		return err
	}
	fmt.Printf("%s Get %d history, filename is %s \n", log.Prefix, len(h.history), filename)
	return nil
}

func (l *Logins) OutPutJson(browser, dir string) error {
	filename := utils.FormatFileName(dir, browser, "password", "json")
	err := writeToJson(filename, l.logins)
	if err != nil {
		log.Error(errWriteToFile)
		return err
	}
	fmt.Printf("%s Get %d passwords, filename is %s \n", log.Prefix, len(l.logins), filename)
	return nil
}

func (c *Cookies) OutPutJson(browser, dir string) error {
	filename := utils.FormatFileName(dir, browser, "cookie", "json")
	err := writeToJson(filename, c.cookies)
	if err != nil {
		log.Error(errWriteToFile)
		return err
	}
	fmt.Printf("%s Get %d cookies, filename is %s \n", log.Prefix, len(c.cookies), filename)
	return nil
}

func writeToJson(filename string, data interface{}) error {
	f, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer func() {
		if err := f.Close(); err != nil {
			log.Error(err)
		}
	}()
	w := new(bytes.Buffer)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "\t")
	err = enc.Encode(data)
	if err != nil {
		log.Debug(err)
		return err
	}
	_, err = f.Write(w.Bytes())
	if err != nil {
		log.Error(err)
		return err
	}
	return nil
}

func (b *Bookmarks) OutPutCsv(browser, dir string) error {
	filename := utils.FormatFileName(dir, browser, "bookmark", "csv")
	if err := writeToCsv(filename, b.bookmarks); err != nil {
		log.Error(errWriteToFile)
		return err
	}
	fmt.Printf("%s Get %d bookmarks, filename is %s \n", log.Prefix, len(b.bookmarks), filename)
	return nil
}

func (h *History) OutPutCsv(browser, dir string) error {
	filename := utils.FormatFileName(dir, browser, "history", "csv")
	if err := writeToCsv(filename, h.history); err != nil {
		log.Error(errWriteToFile)
		return err
	}
	fmt.Printf("%s Get %d history, filename is %s \n", log.Prefix, len(h.history), filename)
	return nil
}

func (l *Logins) OutPutCsv(browser, dir string) error {
	filename := utils.FormatFileName(dir, browser, "password", "csv")
	if err := writeToCsv(filename, l.logins); err != nil {
		log.Error(errWriteToFile)
		return err
	}
	fmt.Printf("%s Get %d passwords, filename is %s \n", log.Prefix, len(l.logins), filename)
	return nil
}

func (c *Cookies) OutPutCsv(browser, dir string) error {
	filename := utils.FormatFileName(dir, browser, "cookie", "csv")
	var tempSlice []cookies
	for _, v := range c.cookies {
		tempSlice = append(tempSlice, v...)
	}
	if err := writeToCsv(filename, tempSlice); err != nil {
		log.Error(errWriteToFile)
		return err
	}
	fmt.Printf("%s Get %d cookies, filename is %s \n", log.Prefix, len(c.cookies), filename)
	return nil
}

func writeToCsv(filename string, data interface{}) error {
	var d []byte
	f, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC|os.O_APPEND, 0644)
	if err != nil {
		log.Errorf("create file %s fail %s", filename, err)
		return err
	}
	defer func() {
		if err := f.Close(); err != nil {
			log.Error(err)
		}
	}()
	_, err = f.Write(utf8Bom)
	if err != nil {
		log.Error(err)
		return err
	}
	d, err = csvutil.Marshal(data)
	if err != nil {
		return err
	}
	_, err = f.Write(d)
	if err != nil {
		log.Error(err)
		return err
	}
	return nil
}
