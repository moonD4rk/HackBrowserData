package extension

import (
	"os"

	"github.com/tidwall/gjson"

	"hack-browser-data/internal/item"
	"hack-browser-data/internal/log"
	"hack-browser-data/internal/utils/fileutil"
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

func (c *ChromiumExtension) Parse(masterKey []byte) error {
	files, err := fileutil.FilesInFolder(item.TempChromiumExtension, manifest)
	if err != nil {
		return err
	}
	defer os.RemoveAll(item.TempChromiumExtension)
	for _, f := range files {
		file, err := fileutil.ReadFile(f)
		if err != nil {
			log.Error("Failed to read file: %s", err)
			continue
		}
		b := gjson.Parse(file)
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

type FirefoxExtension []*extension

func (f *FirefoxExtension) Parse(masterKey []byte) error {
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
