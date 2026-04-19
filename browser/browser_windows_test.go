//go:build windows

package browser

import (
	"testing"

	"github.com/moond4rk/hackbrowserdata/utils/winutil"
)

// TestWinUtilTableCoversABEBrowsers verifies that every Windows browser
// with ABE support in winutil.Table has a matching Storage key in
// platformBrowsers(). A mismatch means adding a new Chromium fork was
// incomplete: either the BrowserConfig row lacks Storage: "<key>", or
// winutil.Table has a stale entry nobody retrieves keys for.
func TestWinUtilTableCoversABEBrowsers(t *testing.T) {
	storages := make(map[string]struct{})
	for _, b := range platformBrowsers() {
		if b.Storage != "" {
			storages[b.Storage] = struct{}{}
		}
	}

	for key, entry := range winutil.Table {
		if entry.ABE == winutil.ABENone {
			continue
		}
		if _, ok := storages[key]; !ok {
			t.Errorf("winutil.Table[%q] declares ABE support but no BrowserConfig.Storage matches — either fix the table or set Storage: %q in platformBrowsers()", key, key)
		}
	}
}
