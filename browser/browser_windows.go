//go:build windows

package browser

import (
	ibrowser "github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/masterkey"
)

// newCredentialInjector hands each ABE browser its executable and dialect; RetrieveAll runs v10 (DPAPI) and,
// when the Locator carries an ABE kind, v20 (reflective injection) — both, since an upgraded profile mixes them.
func newCredentialInjector(_ DiscoverOptions) browserInjector {
	return func(b Browser) {
		km, ok := b.(KeyManager)
		if !ok {
			return
		}
		def, ok := ibrowser.Lookup(km.BrowserKey())
		if !ok {
			return
		}
		if p := def.Platform(); p.ABE != masterkey.ABENone {
			km.SetLocator(masterkey.Locator{Exe: p.Exe, ExeFallbacks: p.ExeFallbacks, ABE: p.ABE})
		}
	}
}
