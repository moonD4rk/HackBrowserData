package safari

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/moond4rk/plist"

	"github.com/moond4rk/hackbrowserdata/types"
)

// Safari keeps extensions in two sibling plists under the container's Safari dir:
//
//	Safari/AppExtensions/Extensions.plist   — legacy App Extensions (XPC-based)
//	Safari/WebExtensions/Extensions.plist   — modern Safari Web Extensions
//
// Both files share the same top-level shape: a dictionary keyed by
// "<bundleID> (<teamID>)". Only WebExtensions carry an `Enabled` field;
// an App Extension that appears in the plist is implicitly enabled.
const (
	safariExtensionsSubdir    = "Safari"
	safariAppExtensionsSubdir = "AppExtensions"
	safariWebExtensionsSubdir = "WebExtensions"
	safariExtensionsPlistFile = "Extensions.plist"
)

// extensionKeyPattern matches the "<bundleID> (<teamID>)" key format Safari uses.
var extensionKeyPattern = regexp.MustCompile(`^(\S+)\s+\(([^)]+)\)$`)

// safariExtension mirrors the per-extension dict value in Extensions.plist.
// Only fields that map onto types.ExtensionEntry are decoded; richer fields
// (Permissions, AccessibleOrigins, …) are intentionally ignored for the
// minimum implementation.
type safariExtension struct {
	Enabled *bool `plist:"Enabled"`
}

// extractExtensions reads both AppExtensions/Extensions.plist and
// WebExtensions/Extensions.plist from the profile's Safari container and
// returns the merged list, sorted by key for deterministic output.
// A missing plist on either side is skipped silently.
func extractExtensions(container string) ([]types.ExtensionEntry, error) {
	records, err := readSafariExtensions(container)
	if err != nil {
		return nil, err
	}

	extensions := make([]types.ExtensionEntry, 0, len(records))
	for _, r := range records {
		extensions = append(extensions, types.ExtensionEntry{
			Name:    r.bundleID,
			ID:      r.key,
			Enabled: r.enabled,
		})
	}
	return extensions, nil
}

func countExtensions(container string) (int, error) {
	records, err := readSafariExtensions(container)
	if err != nil {
		return 0, err
	}
	return len(records), nil
}

type extensionRecord struct {
	key      string
	bundleID string
	enabled  bool
}

func readSafariExtensions(container string) ([]extensionRecord, error) {
	safariDir := filepath.Join(container, safariExtensionsSubdir)
	var all []extensionRecord
	for _, sub := range []string{safariAppExtensionsSubdir, safariWebExtensionsSubdir} {
		p := filepath.Join(safariDir, sub, safariExtensionsPlistFile)
		records, err := decodeSafariExtensionsPlist(p)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
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

// bundleIDFromExtensionKey extracts the bundle ID from a "<bundleID> (<teamID>)"
// key; falls back to the trimmed full key when the format doesn't match.
func bundleIDFromExtensionKey(key string) string {
	if m := extensionKeyPattern.FindStringSubmatch(key); m != nil {
		return m[1]
	}
	return strings.TrimSpace(key)
}
