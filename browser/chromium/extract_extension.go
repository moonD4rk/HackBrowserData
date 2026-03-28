package chromium

import (
	"fmt"
	"os"

	"github.com/tidwall/gjson"

	"github.com/moond4rk/hackbrowserdata/types"
)

func extractExtensions(path string) ([]types.ExtensionEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Try known JSON paths for extension settings
	settingKeys := []string{
		"extensions.settings",
		"settings.extensions",
		"settings.settings",
	}
	var settings gjson.Result
	for _, key := range settingKeys {
		settings = gjson.GetBytes(data, key)
		if settings.Exists() {
			break
		}
	}
	if !settings.Exists() {
		return nil, fmt.Errorf("cannot find extensions in settings")
	}

	var extensions []types.ExtensionEntry
	settings.ForEach(func(id, ext gjson.Result) bool {
		// Skip system/component extensions
		// https://source.chromium.org/chromium/chromium/src/+/main:extensions/common/mojom/manifest.mojom
		location := ext.Get("location").Int()
		if location == 5 || location == 10 {
			return true
		}

		manifest := ext.Get("manifest")
		if !manifest.Exists() {
			return true
		}

		extensions = append(extensions, types.ExtensionEntry{
			Name:        manifest.Get("name").String(),
			ID:          id.String(),
			Description: manifest.Get("description").String(),
			Version:     manifest.Get("version").String(),
		})
		return true
	})

	return extensions, nil
}
