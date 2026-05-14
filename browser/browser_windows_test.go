//go:build windows

package browser

import (
	"testing"

	"github.com/moond4rk/hackbrowserdata/utils/winutil"
)

// TestWinUtilTableCoversABEBrowsers verifies that the set of Windows browsers
// with WindowsABE: true in platformBrowsers() exactly matches the set of
// winutil.Table entries that declare ABE support (keyed by BrowserConfig.Key ==
// winutil.Entry.Key). A mismatch means adding a new Chromium fork was
// incomplete: either a BrowserConfig row is missing WindowsABE: true, or
// winutil.Table has a stale/missing entry.
func TestWinUtilTableCoversABEBrowsers(t *testing.T) {
	abeConfigs := make(map[string]struct{})
	for _, b := range platformBrowsers() {
		if b.WindowsABE {
			abeConfigs[b.Key] = struct{}{}
		}
	}

	abeTable := make(map[string]struct{})
	for key, entry := range winutil.Table {
		if entry.Key != key {
			t.Errorf("winutil.Table[%q].Key = %q; map key and Entry.Key must match (winutil.Entry doc invariant)", key, entry.Key)
		}
		if entry.ABE != winutil.ABENone {
			abeTable[key] = struct{}{}
		}
	}

	for key := range abeTable {
		if _, ok := abeConfigs[key]; !ok {
			t.Errorf("winutil.Table[%q] declares ABE support but no BrowserConfig with Key %q sets WindowsABE: true — either fix the table or set WindowsABE: true in platformBrowsers()", key, key)
		}
	}
	for key := range abeConfigs {
		if _, ok := abeTable[key]; !ok {
			t.Errorf("BrowserConfig with Key %q sets WindowsABE: true but winutil.Table[%q] is missing or declares no ABE — add the table entry", key, key)
		}
	}
}
