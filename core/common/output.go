package common

import (
	"bytes"
	"encoding/json"
	"fmt"
	"hack-browser-data/log"
	"hack-browser-data/utils"
	"os"

	"github.com/jszwec/csvutil"
)

var utf8Bom = []byte{239, 187, 191}

func (b *Bookmarks) OutPutJson(browser, dir string) error {
	filename := utils.FormatFileName(dir, browser, "bookmark", "json")
	file, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC|os.O_APPEND, 0644)
	defer func() {
		if err := file.Close(); err != nil {
			log.Error(err)
		}
	}()

	if err != nil {
		log.Errorf("create file %s fail", filename)
		return err
	}
	w := new(bytes.Buffer)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "\t")
	enc.Encode(b.bookmarks)
	_, err = file.Write(w.Bytes())
	if err != nil {
		log.Error(err)
		return err
	}
	fmt.Printf("%s Get %d bookmarks, filename is %s \n", log.Prefix, len(b.bookmarks), filename)
	return nil
}

func (h *History) OutPutJson(browser, dir string) error {
	filename := utils.FormatFileName(dir, browser, "history", "json")
	file, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC|os.O_APPEND, 0644)
	defer func() {
		if err := file.Close(); err != nil {
			log.Error(err)
		}
	}()
	if err != nil {
		log.Errorf("create file %s fail", filename)
		return err
	}
	w := new(bytes.Buffer)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "\t")
	err = enc.Encode(h.history)
	if err != nil {
		log.Debug(err)
	}
	_, err = file.Write(w.Bytes())
	if err != nil {
		log.Error(err)
		return err
	}
	fmt.Printf("%s Get %d history, filename is %s \n", log.Prefix, len(h.history), filename)
	return nil
}

func (l *Logins) OutPutJson(browser, dir string) error {
	filename := utils.FormatFileName(dir, browser, "password", "json")
	file, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC|os.O_APPEND, 0644)
	defer func() {
		if err := file.Close(); err != nil {
			log.Error(err)
		}
	}()
	if err != nil {
		log.Errorf("create file %s fail", filename)
		return err
	}
	w := new(bytes.Buffer)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "\t")
	err = enc.Encode(l.logins)
	if err != nil {
		log.Debug(err)
	}
	_, err = file.Write(w.Bytes())
	if err != nil {
		log.Error(err)
		return err
	}
	fmt.Printf("%s Get %d history, filename is %s \n", log.Prefix, len(l.logins), filename)
	return nil
}

func (c *Cookies) OutPutJson(browser, dir string) error {
	filename := utils.FormatFileName(dir, browser, "cookie", "json")
	file, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC|os.O_APPEND, 0644)
	defer func() {
		if err := file.Close(); err != nil {
			log.Error(err)
		}
	}()
	if err != nil {
		log.Errorf("create file %s fail", filename)
		return err
	}
	w := new(bytes.Buffer)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "\t")
	err = enc.Encode(c.cookies)
	if err != nil {
		log.Debug(err)
		return err
	}
	_, err = file.Write(w.Bytes())
	if err != nil {
		log.Error(err)
		return err
	}
	fmt.Printf("%s Get %d history, filename is %s \n", log.Prefix, len(c.cookies), filename)
	return nil
}

func (b *Bookmarks) OutPutCsv(browser, dir string) error {
	filename := utils.FormatFileName(dir, browser, "bookmark", "csv")
	file, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC|os.O_APPEND, 0644)
	defer func() {
		if err := file.Close(); err != nil {
			log.Error(err)
		}
	}()
	if err != nil {
		log.Errorf("create file %s fail %s", filename, err)
		return err
	}
	file.Write(utf8Bom)
	data, err := csvutil.Marshal(b.bookmarks)
	if err != nil {
		return err
	}
	file.Write(data)
	fmt.Printf("%s Get %d bookmarks, filename is %s \n", log.Prefix, len(b.bookmarks), filename)
	return nil
}

func (h *History) OutPutCsv(browser, dir string) error {
	filename := utils.FormatFileName(dir, browser, "history", "csv")
	file, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC|os.O_APPEND, 0644)
	defer func() {
		if err := file.Close(); err != nil {
			log.Error(err)
		}
	}()
	if err != nil {
		log.Errorf("create file %s fail %s", filename, err)
		return err
	}
	file.Write(utf8Bom)
	data, err := csvutil.Marshal(h.history)
	if err != nil {
		return err
	}
	file.Write(data)
	fmt.Printf("%s Get %d bookmarks, filename is %s \n", log.Prefix, len(h.history), filename)
	return nil
}

func (l *Logins) OutPutCsv(browser, dir string) error {
	filename := utils.FormatFileName(dir, browser, "password", "csv")
	file, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC|os.O_APPEND, 0644)
	defer func() {
		if err := file.Close(); err != nil {
			log.Error(err)
		}
	}()
	if err != nil {
		log.Errorf("create file %s fail %s", filename, err)
		return err
	}
	file.Write(utf8Bom)
	data, err := csvutil.Marshal(l.logins)
	if err != nil {
		return err
	}
	file.Write(data)
	fmt.Printf("%s Get %d bookmarks, filename is %s \n", log.Prefix, len(l.logins), filename)
	return nil
}

func (c *Cookies) OutPutCsv(browser, dir string) error {
	filename := utils.FormatFileName(dir, browser, "cookie", "csv")
	file, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC|os.O_APPEND, 0644)
	defer func() {
		if err := file.Close(); err != nil {
			log.Error(err)
		}
	}()
	if err != nil {
		log.Errorf("create file %s fail", filename)
		return err
	}
	var tempSlice []cookies
	for _, v := range c.cookies {
		tempSlice = append(tempSlice, v...)
	}
	file.Write(utf8Bom)
	data, err := csvutil.Marshal(tempSlice)
	file.Write(data)
	fmt.Printf("%s Get %d cookies, filename is %s \n", log.Prefix, len(c.cookies), filename)
	return nil
}
