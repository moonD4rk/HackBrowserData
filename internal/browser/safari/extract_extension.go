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

// Safari keeps App Extensions and Web Extensions in two plists keyed by "<bundleID> (<teamID>)".
// Only Web Extensions carry Enabled; a listed App Extension is enabled.
var extensionKeyPattern = regexp.MustCompile(`^(\S+)\s+\(([^)]+)\)$`)

// Only the fields record.Extension needs are decoded.
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
	for _, r := range records {
		extensions = append(extensions, record.Extension{
			Name:    r.bundleID,
			ID:      r.key,
			Enabled: r.enabled,
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
	for _, p := range paths {
		records, err := decodeSafariExtensionsPlist(p)
		if err != nil {
			return nil, err
		}
		all = append(all, records...)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].key < all[j].key })
	return all, nil
}

func decodeSafariExtensionsPlist(path string) ([]extensionRecord, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var decoded map[string]safariExtension
	if err := plist.NewDecoder(f).Decode(&decoded); err != nil {
		return nil, fmt.Errorf("decode extensions %s: %w", path, err)
	}

	records := make([]extensionRecord, 0, len(decoded))
	for key, ext := range decoded {
		enabled := true
		if ext.Enabled != nil {
			enabled = *ext.Enabled
		}
		records = append(records, extensionRecord{
			key:      key,
			bundleID: bundleIDFromExtensionKey(key),
			enabled:  enabled,
		})
	}
	return records, nil
}

// A key that does not match the pattern falls back to the whole trimmed key.
func bundleIDFromExtensionKey(key string) string {
	if m := extensionKeyPattern.FindStringSubmatch(key); m != nil {
		return m[1]
	}
	return strings.TrimSpace(key)
}
