package safari

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/moond4rk/plist"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/record"
)

// extensionKeyPattern parses the "<bundleID> (<teamID>)" keys of the App Extensions and Web Extensions plists.
// Only Web Extensions carry Enabled; a listed App Extension is enabled.
var extensionKeyPattern = regexp.MustCompile(`^(\S+)\s+\(([^)]+)\)$`)

// safariExtension decodes only the fields record.Extension needs.
type safariExtension struct {
	Enabled *bool `plist:"Enabled"`
}

// extractExtensions merges the plists in paths, sorted by key for deterministic output.
func extractExtensions(paths []string, out browser.Collector) error {
	records, err := readSafariExtensions(paths)
	if err != nil {
		return err
	}

	extensions := make([]record.Extension, 0, len(records))
	for _, entry := range records {
		extensions = append(extensions, record.Extension{
			Name:    entry.bundleID,
			ID:      entry.key,
			Enabled: entry.enabled,
		})
	}
	emit(out, extensions)
	return nil
}

type extensionRecord struct {
	key      string
	bundleID string
	enabled  bool
}

func readSafariExtensions(paths []string) ([]extensionRecord, error) {
	var all []extensionRecord
	for _, plistPath := range paths {
		records, err := decodeSafariExtensionsPlist(plistPath)
		if err != nil {
			return nil, err
		}
		all = append(all, records...)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].key < all[j].key })
	return all, nil
}

func decodeSafariExtensionsPlist(path string) ([]extensionRecord, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var decoded map[string]safariExtension
	if err := plist.NewDecoder(file).Decode(&decoded); err != nil {
		return nil, fmt.Errorf("decode extensions %s: %w", path, err)
	}

	records := make([]extensionRecord, 0, len(decoded))
	for key, extension := range decoded {
		enabled := true
		if extension.Enabled != nil {
			enabled = *extension.Enabled
		}
		records = append(records, extensionRecord{
			key:      key,
			bundleID: bundleIDFromExtensionKey(key),
			enabled:  enabled,
		})
	}
	return records, nil
}

// bundleIDFromExtensionKey falls back to the whole trimmed key when it does not match the pattern.
func bundleIDFromExtensionKey(key string) string {
	if match := extensionKeyPattern.FindStringSubmatch(key); match != nil {
		return match[1]
	}
	return strings.TrimSpace(key)
}
