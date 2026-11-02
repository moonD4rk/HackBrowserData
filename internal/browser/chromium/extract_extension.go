package chromium

import (
	"fmt"
	"os"

	"github.com/tidwall/gjson"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/record"
)

var defaultExtensionKeys = []string{
	"extensions.settings",
	"settings.extensions",
	"settings.settings",
}

func extractExtensions(path string, out browser.Collector) error {
	return extractExtensionsWithKeys(path, defaultExtensionKeys, out)
}

// extractExtensionsWithKeys takes the JSON paths, because variants such as Opera keep extension settings elsewhere.
func extractExtensionsWithKeys(path string, keys []string, out browser.Collector) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var settings gjson.Result
	for _, key := range keys {
		settings = gjson.GetBytes(data, key)
		if settings.Exists() {
			break
		}
	}
	if !settings.Exists() {
		return fmt.Errorf("cannot find extensions in settings")
	}

	var extensions []record.Extension
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

		extensions = append(extensions, record.Extension{
			Name:        manifest.Get("name").String(),
			ID:          id.String(),
			Description: manifest.Get("description").String(),
			Version:     manifest.Get("version").String(),
			HomepageURL: manifest.Get("homepage_url").String(),
			Enabled:     isExtensionEnabled(extension),
		})
		return true
	})

	browser.AddAll(out, extensions)
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

func extractOperaExtensions(path string, out browser.Collector) error {
	return extractExtensionsWithKeys(path, []string{"extensions.opsettings"}, out)
}
