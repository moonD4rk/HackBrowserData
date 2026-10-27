//go:build windows

package browser

import (
	"testing"

	ibrowser "github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/masterkey"
)

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
	def, ok := ibrowser.Lookup("chrome")
	if !ok {
		t.Fatal("chrome is not in the browser table")
	}
	want := def.Platform()
	if chrome.got.Exe != want.Exe || chrome.got.ABE != want.ABE || len(chrome.got.ExeFallbacks) != len(want.ExeFallbacks) {
		t.Errorf("chrome Locator = %+v, want exe %q, kind %v, %d fallbacks", *chrome.got, want.Exe, want.ABE, len(want.ExeFallbacks))
	}

	opera := &locatorRecorder{mockChromiumBrowser: mockChromiumBrowser{key: "opera"}}
	inject(opera)
	if opera.got != nil {
		t.Errorf("opera has no ABE, so it keeps DPAPI only; got Locator %+v", *opera.got)
	}
}
