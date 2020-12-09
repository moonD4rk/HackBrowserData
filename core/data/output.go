package data

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"hack-browser-data/utils"

	"github.com/jszwec/csvutil"
)

var (
	utf8Bom = []byte{239, 187, 191}
)

func (b *bookmarks) outPutJson(browser, dir string) error {
	filename := utils.FormatFileName(dir, browser, "bookmark", "json")
	sort.Slice(b.bookmarks, func(i, j int) bool {
		return b.bookmarks[i].ID < b.bookmarks[j].ID
	})
	err := writeToJson(filename, b.bookmarks)
	if err != nil {
		return err
	}
	fmt.Printf("%s Get %d bookmarks, filename is %s \n", utils.Prefix, len(b.bookmarks), filename)
	return nil
}

func (h *historyData) outPutJson(browser, dir string) error {
	filename := utils.FormatFileName(dir, browser, "history", "json")
	sort.Slice(h.history, func(i, j int) bool {
		return h.history[i].VisitCount > h.history[j].VisitCount
	})
	err := writeToJson(filename, h.history)
	if err != nil {
		return err
	}
	fmt.Printf("%s Get %d history, filename is %s \n", utils.Prefix, len(h.history), filename)
	return nil
}

func (p *passwords) outPutJson(browser, dir string) error {
	filename := utils.FormatFileName(dir, browser, "password", "json")
	err := writeToJson(filename, p.logins)
	if err != nil {
		return err
	}
	fmt.Printf("%s Get %d passwords, filename is %s \n", utils.Prefix, len(p.logins), filename)
	return nil
}

func (c *cookies) outPutJson(browser, dir string) error {
	filename := utils.FormatFileName(dir, browser, "cookie", "json")
	err := writeToJson(filename, c.cookies)
	if err != nil {
		return err
	}
	fmt.Printf("%s Get %d cookies, filename is %s \n", utils.Prefix, len(c.cookies), filename)
	return nil
}

func (credit *creditcards) outPutJson(browser, dir string) error {
	filename := utils.FormatFileName(dir, browser, "credit", "json")
	err := writeToJson(filename, credit.cards)
	if err != nil {
		return err
	}
	fmt.Printf("%s Get %d cards, filename is %s \n", utils.Prefix, len(credit.cards), filename)
	return nil
}

func writeToJson(filename string, data interface{}) error {
	f, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	w := new(bytes.Buffer)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "\t")
	err = enc.Encode(data)
	if err != nil {
		return err
	}
	_, err = f.Write(w.Bytes())
	if err != nil {
		return err
	}
	return nil
}

func (b *bookmarks) outPutCsv(browser, dir string) error {
	filename := utils.FormatFileName(dir, browser, "bookmark", "csv")
	if err := writeToCsv(filename, b.bookmarks); err != nil {
		return err
	}
	fmt.Printf("%s Get %d bookmarks, filename is %s \n", utils.Prefix, len(b.bookmarks), filename)
	return nil
}

func (h *historyData) outPutCsv(browser, dir string) error {
	filename := utils.FormatFileName(dir, browser, "history", "csv")
	if err := writeToCsv(filename, h.history); err != nil {
		return err
	}
	fmt.Printf("%s Get %d history, filename is %s \n", utils.Prefix, len(h.history), filename)
	return nil
}

func (p *passwords) outPutCsv(browser, dir string) error {
	filename := utils.FormatFileName(dir, browser, "password", "csv")
	if err := writeToCsv(filename, p.logins); err != nil {
		return err
	}
	fmt.Printf("%s Get %d passwords, filename is %s \n", utils.Prefix, len(p.logins), filename)
	return nil
}

func (c *cookies) outPutCsv(browser, dir string) error {
	filename := utils.FormatFileName(dir, browser, "cookie", "csv")
	var tempSlice []cookie
	for _, v := range c.cookies {
		tempSlice = append(tempSlice, v...)
	}
	if err := writeToCsv(filename, tempSlice); err != nil {
		return err
	}
	fmt.Printf("%s Get %d cookies, filename is %s \n", utils.Prefix, len(c.cookies), filename)
	return nil
}

func (credit *creditcards) outPutCsv(browser, dir string) error {
	filename := utils.FormatFileName(dir, browser, "credit", "csv")
	var tempSlice []card
	for _, v := range credit.cards {
		tempSlice = append(tempSlice, v...)
	}
	if err := writeToCsv(filename, tempSlice); err != nil {
		return err
	}
	fmt.Printf("%s Get %d cards, filename is %s \n", utils.Prefix, len(credit.cards), filename)
	return nil
}

func writeToCsv(filename string, data interface{}) error {
	var d []byte
	f, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(utf8Bom)
	if err != nil {
		return err
	}
	d, err = csvutil.Marshal(data)
	if err != nil {
		return err
	}
	_, err = f.Write(d)
	if err != nil {
		return err
	}
	return nil
}

func (b *bookmarks) outPutConsole() {
	for _, v := range b.bookmarks {
		fmt.Printf("%+v\n", v)
	}
}

func (c *cookies) outPutConsole() {
	for host, value := range c.cookies {
		fmt.Printf("%s\n%+v\n", host, value)
	}
}

func (h *historyData) outPutConsole() {
	for _, v := range h.history {
		fmt.Printf("%+v\n", v)
	}
}

func (p *passwords) outPutConsole() {
	for _, v := range p.logins {
		fmt.Printf("%+v\n", v)
	}
}

func (credit *creditcards) outPutConsole() {
	for _, v := range credit.cards {
		fmt.Printf("%+v\n", v)
	}
}
