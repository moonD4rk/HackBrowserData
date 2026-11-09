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

	for _, value := range gjson.GetBytes(data, "addons").Array() {
		// Only include user-installed extensions
		// https://searchfox.org/mozilla-central/source/toolkit/mozapps/extensions/internal/XPIDatabase.jsm#157
		if value.Get("location").String() != "app-profile" {
			continue
		}

		out.Add(&record.Extension{
			Name:        value.Get("defaultLocale.name").String(),
			ID:          value.Get("id").String(),
			Description: value.Get("defaultLocale.description").String(),
			Version:     value.Get("version").String(),
			HomepageURL: value.Get("defaultLocale.homepageURL").String(),
			Enabled:     value.Get("active").Bool(),
		})
	}
	return nil
}
