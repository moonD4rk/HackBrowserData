package extension

import (
	"os"

	"github.com/tidwall/gjson"

	"github.com/moond4rk/HackBrowserData/item"
	"github.com/moond4rk/HackBrowserData/log"
	"github.com/moond4rk/HackBrowserData/utils/fileutil"
)

type ChromiumExtension []*extension

type extension struct {
	Name        string
	Description string
	Version     string
	HomepageURL string
}

const (
	manifest = "manifest.json"
)

func (c *ChromiumExtension) Parse(_ []byte) error {
	files, err := fileutil.FilesInFolder(item.TempChromiumExtension, manifest)
	if err != nil {
		return err
	}
	defer os.RemoveAll(item.TempChromiumExtension)
	for _, f := range files {
		content, err := fileutil.ReadFile(f)
		if err != nil {
			log.Error("Failed to read content: %s", err)
			continue
		}
		b := gjson.Parse(content)
		*c = append(*c, &extension{
			Name:        b.Get("name").String(),
			Description: b.Get("description").String(),
			Version:     b.Get("version").String(),
			HomepageURL: b.Get("homepage_url").String(),
		})
	}
	return nil
}

func (c *ChromiumExtension) Name() string {
	return "extension"
}

func (c *ChromiumExtension) Len() int {
	return len(*c)
}

type FirefoxExtension []*extension

func (f *FirefoxExtension) Parse(_ []byte) error {
	s, err := fileutil.ReadFile(item.TempFirefoxExtension)
	if err != nil {
		return err
	}
	defer os.Remove(item.TempFirefoxExtension)
	j := gjson.Parse(s)
	for _, v := range j.Get("addons").Array() {
		*f = append(*f, &extension{
			Name:        v.Get("defaultLocale.name").String(),
			Description: v.Get("defaultLocale.description").String(),
			Version:     v.Get("version").String(),
			HomepageURL: v.Get("defaultLocale.homepageURL").String(),
		})
	}
	return nil
}

func (f *FirefoxExtension) Name() string {
	return "extension"
}

func (f *FirefoxExtension) Len() int {
	return len(*f)
}
