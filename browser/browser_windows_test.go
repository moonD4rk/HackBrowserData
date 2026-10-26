//go:build windows

package browser

import (
	"testing"

	"github.com/moond4rk/hackbrowserdata/internal/masterkey"
)

// TestABETargetsCoverABEBrowsers verifies that the browsers with WindowsABE: true in platformBrowsers()
// are exactly the keys in abeTargets. A mismatch means adding a Chromium fork was incomplete: either a
// BrowserConfig row is missing WindowsABE: true, or abeTargets has a stale or missing entry.
func TestABETargetsCoverABEBrowsers(t *testing.T) {
	abeConfigs := make(map[string]struct{})
	for _, b := range platformBrowsers() {
		if b.WindowsABE {
			abeConfigs[b.Key] = struct{}{}
		}
	}

	for key, target := range abeTargets {
		if target.exe == "" {
			t.Errorf("abeTargets[%q] has no executable name", key)
		}
		if target.kind == masterkey.ABENone {
			t.Errorf("abeTargets[%q] declares no ABE kind", key)
		}
		if _, ok := abeConfigs[key]; !ok {
			t.Errorf("abeTargets[%q] has an entry but no BrowserConfig with Key %q sets WindowsABE: true", key, key)
		}
	}
	for key := range abeConfigs {
		if _, ok := abeTargets[key]; !ok {
			t.Errorf("BrowserConfig with Key %q sets WindowsABE: true but abeTargets[%q] is missing", key, key)
		}
	}
}

type locatorRecorder struct {
	mockChromiumBrowser
	got *masterkey.Locator
}

func (r *locatorRecorder) SetLocator(loc masterkey.Locator) { r.got = &loc }

// A lost ABE hand-off silently disables v20 decryption on Windows, so the injector's wiring is pinned here.
func TestCredentialInjector_HandsABEBrowsersTheirTarget(t *testing.T) {
	inject := newCredentialInjector(DiscoverOptions{})

	chrome := &locatorRecorder{mockChromiumBrowser: mockChromiumBrowser{key: "chrome"}}
	inject(chrome)
	if chrome.got == nil {
		t.Fatal("chrome got no Locator")
	}
	want := abeTargets["chrome"]
	if chrome.got.Exe != want.exe || chrome.got.ABE != want.kind || len(chrome.got.ExeFallbacks) != len(want.fallbacks) {
		t.Errorf("chrome Locator = %+v, want exe %q, kind %v, %d fallbacks", *chrome.got, want.exe, want.kind, len(want.fallbacks))
	}

	opera := &locatorRecorder{mockChromiumBrowser: mockChromiumBrowser{key: "opera"}}
	inject(opera)
	if opera.got != nil {
		t.Errorf("opera has no ABE, so it keeps DPAPI only; got Locator %+v", *opera.got)
	}
}
