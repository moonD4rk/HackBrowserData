package firefox

import (
	"os"

	"github.com/tidwall/gjson"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/record"
)

func extractExtensions(path string, out browser.Collector) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var extensions []record.Extension
	for _, v := range gjson.GetBytes(data, "addons").Array() {
		// Only include user-installed extensions
		// https://searchfox.org/mozilla-central/source/toolkit/mozapps/extensions/internal/XPIDatabase.jsm#157
		if v.Get("location").String() != "app-profile" {
			continue
		}

		extensions = append(extensions, record.Extension{
			Name:        v.Get("defaultLocale.name").String(),
			ID:          v.Get("id").String(),
			Description: v.Get("defaultLocale.description").String(),
			Version:     v.Get("version").String(),
			HomepageURL: v.Get("defaultLocale.homepageURL").String(),
			Enabled:     v.Get("active").Bool(),
		})
	}
	emit(out, extensions)
	return nil
}
