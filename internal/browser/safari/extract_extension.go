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

// extensionState decodes only the fields record.Extension needs.
type extensionState struct {
	Enabled *bool `plist:"Enabled"`
}

// extractExtensions merges the plists in paths, sorted by key for deterministic output.
func extractExtensions(paths []string, out browser.Collector) error {
	records, err := readExtensions(paths)
	if err != nil {
		return err
	}

	for _, entry := range records {
		out.Add(&record.Extension{
			Name:    entry.bundleID,
			ID:      entry.key,
			Enabled: entry.enabled,
		})
	}
	return nil
}

type extensionRecord struct {
	key      string
	bundleID string
	enabled  bool
}

func readExtensions(paths []string) ([]extensionRecord, error) {
	var all []extensionRecord
	for _, plistPath := range paths {
		records, err := decodeExtensionsPlist(plistPath)
		if err != nil {
			return nil, err
		}
		all = append(all, records...)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].key < all[j].key })
	return all, nil
}

func decodeExtensionsPlist(path string) ([]extensionRecord, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var decoded map[string]extensionState
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
