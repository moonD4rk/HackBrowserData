package browser

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var platformOf = map[string]func(Browser) Platform{
	"darwin":  func(browser Browser) Platform { return browser.Darwin },
	"linux":   func(browser Browser) Platform { return browser.Linux },
	"windows": func(browser Browser) Platform { return browser.Windows },
}

func namesOn(platform func(Browser) Platform) []string {
	var names []string
	for _, browser := range browsers {
		if len(platform(browser).UserDataDirs) > 0 {
			names = append(names, browser.Name)
		}
	}
	return names
}

func TestSupported_ListsThisOSRowsInTableOrder(t *testing.T) {
	var names []string
	for _, browser := range Supported() {
		names = append(names, browser.Name)
	}
	assert.Equal(t, namesOn(Browser.Platform), names)
}

func TestLookup_FindsEveryRowWhateverTheOS(t *testing.T) {
	for _, want := range browsers {
		got, ok := Lookup(want.Name)
		require.True(t, ok, want.Name)
		assert.Equal(t, want, got)
	}
	_, ok := Lookup("netscape")
	assert.False(t, ok)
}

func TestNames_AreSupportedSorted(t *testing.T) {
	names := Names()
	assert.True(t, sort.StringsAreSorted(names), "the -b help has always listed them sorted")
	assert.ElementsMatch(t, namesOn(Browser.Platform), names)
}
