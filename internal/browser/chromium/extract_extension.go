package chromium

import (
	"errors"
	"io/fs"
	"os"

	"github.com/tidwall/gjson"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/masterkey"
	"github.com/moond4rk/hackbrowserdata/internal/record"
)

var extensionSettingsKeys = []string{
	"extensions.settings",
	"settings.extensions",
	"settings.settings",
}

var errNoExtensionList = errors.New("cannot find extensions in settings")

// newExtensionExtractor takes the settings keys the extension list may sit under, since forks such as Opera move it.
// It falls back to Preferences, where Linux builds and forks such as 360 keep the list.
func newExtensionExtractor(settingsKeys ...string) extractor {
	sources := []browser.Source{securePreferences, preferences}
	return extractor{
		sources: sources,
		extract: func(captured browser.ProfileSnapshot, _ masterkey.Set, out browser.Collector) error {
			for _, source := range sources {
				path, err := captured.Path(source)
				if errors.Is(err, fs.ErrNotExist) {
					continue
				}
				if err == nil {
					err = extractExtensions(path, settingsKeys, out)
				}
				if !errors.Is(err, errNoExtensionList) {
					return err
				}
			}
			return errNoExtensionList
		},
	}
}

func extractExtensions(path string, settingsKeys []string, out browser.Collector) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var settings gjson.Result
	for _, key := range settingsKeys {
		settings = gjson.GetBytes(data, key)
		if settings.Exists() {
			break
		}
	}
	if !settings.Exists() {
		return errNoExtensionList
	}

	settings.ForEach(func(id, extension gjson.Result) bool {
		// Skip system/component extensions
		// https://source.chromium.org/chromium/chromium/src/+/main:extensions/common/mojom/manifest.mojom
		location := extension.Get("location").Int()
		if location == 5 || location == 10 {
			return true
		}

		manifest := extension.Get("manifest")
		if !manifest.Exists() {
			return true
		}

		out.Add(&record.Extension{
			Name:        manifest.Get("name").String(),
			ID:          id.String(),
			Description: manifest.Get("description").String(),
			Version:     manifest.Get("version").String(),
			HomepageURL: manifest.Get("homepage_url").String(),
			Enabled:     isExtensionEnabled(extension),
		})
		return true
	})

	return nil
}

// isExtensionEnabled reads disable_reasons (empty means enabled), or in older Chrome state (1 means enabled).
func isExtensionEnabled(extension gjson.Result) bool {
	reasons := extension.Get("disable_reasons")
	if reasons.Exists() {
		return reasons.IsArray() && len(reasons.Array()) == 0
	}
	return extension.Get("state").Int() == 1
}
