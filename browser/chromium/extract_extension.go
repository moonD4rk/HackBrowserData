package chromium

import (
	"fmt"
	"os"

	"github.com/tidwall/gjson"

	"github.com/moond4rk/hackbrowserdata/types"
)

// defaultExtensionKeys are the JSON paths tried for standard Chromium browsers.
var defaultExtensionKeys = []string{
	"extensions.settings",
	"settings.extensions",
	"settings.settings",
}

func extractExtensions(path string) ([]types.ExtensionEntry, error) {
	return extractExtensionsWithKeys(path, defaultExtensionKeys)
}

// extractExtensionsWithKeys reads Secure Preferences and looks for extension
// settings under the given JSON key paths. This allows browser variants
// (e.g. Opera with "extensions.opsettings") to reuse the same parsing logic.
func extractExtensionsWithKeys(path string, keys []string) ([]types.ExtensionEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var settings gjson.Result
	for _, key := range keys {
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
			HomepageURL: manifest.Get("homepage_url").String(),
			Enabled:     isExtensionEnabled(ext),
		})
		return true
	})

	return extensions, nil
}

// isExtensionEnabled checks whether an extension is enabled.
// Modern Chrome uses disable_reasons (array): empty [] = enabled, non-empty [1] = disabled.
// Older Chrome uses state (int): 1 = enabled.
func isExtensionEnabled(ext gjson.Result) bool {
	reasons := ext.Get("disable_reasons")
	if reasons.Exists() {
		return reasons.IsArray() && len(reasons.Array()) == 0
	}
	return ext.Get("state").Int() == 1
}

// extractOperaExtensions extracts extensions from Opera's Secure Preferences,
// which stores extension data under "extensions.opsettings" instead of the
// standard "extensions.settings".
func extractOperaExtensions(path string) ([]types.ExtensionEntry, error) {
	return extractExtensionsWithKeys(path, []string{"extensions.opsettings"})
}
