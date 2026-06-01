package browser

import (
	"runtime"

	"github.com/moond4rk/hackbrowserdata/log"
	"github.com/moond4rk/hackbrowserdata/masterkey"
)

// BuildDump exports one Vault per installation (Firefox/Safari, lacking KeyManager, are skipped).
// Partial results are kept — a Chrome 127+ profile mixes v10+v20, so a v20-only failure must not
// discard a usable v10 key.
func BuildDump(browsers []Browser) masterkey.Dump {
	dump := masterkey.NewDump()
	for _, b := range browsers {
		km, ok := b.(KeyManager)
		if !ok {
			continue
		}
		mk, err := km.ExportKeys()
		if err != nil {
			status := "partial"
			if !mk.HasAny() {
				status = "failed"
			}
			log.Warnf("dump-keys: %s %s: %v", b.BrowserName(), status, err)
		}
		if !mk.HasAny() {
			continue
		}
		dump.Vaults = append(dump.Vaults, masterkey.Vault{
			Browser:     b.BrowserName(),
			UserDataDir: b.UserDataDir(),
			Profiles:    profileNames(b),
			Keys:        mk,
		})
	}
	return dump
}

func profileNames(b Browser) []string {
	profiles := b.Profiles()
	names := make([]string, 0, len(profiles))
	for _, p := range profiles {
		names = append(names, p.Name)
	}
	return names
}

// ApplyDump overlays StaticRetrievers from dump onto matching installations (Firefox/Safari skipped).
// Match is by (BrowserName, UserDataDir); on miss — commonly a cross-host path mismatch (Windows vs
// POSIX, or a relocated dir via -p) — it falls back to the sole vault for that browser name. No match
// → warn and leave the platform retrievers in place.
func ApplyDump(browsers []Browser, dump masterkey.Dump) {
	if dump.Host.OS != "" && dump.Host.OS != runtime.GOOS {
		log.Infof("apply-keys: dump created on %s/%s; current host is %s/%s",
			dump.Host.OS, dump.Host.Arch, runtime.GOOS, runtime.GOARCH)
	}
	vaultIndex := make(map[string]*masterkey.Vault, len(dump.Vaults))
	vaultsByBrowser := make(map[string][]*masterkey.Vault)
	for i := range dump.Vaults {
		v := &dump.Vaults[i]
		vaultIndex[v.Browser+"|"+v.UserDataDir] = v
		vaultsByBrowser[v.Browser] = append(vaultsByBrowser[v.Browser], v)
	}
	for _, b := range browsers {
		km, ok := b.(KeyManager)
		if !ok {
			continue
		}
		v, found := vaultIndex[b.BrowserName()+"|"+b.UserDataDir()]
		if !found {
			if candidates := vaultsByBrowser[b.BrowserName()]; len(candidates) == 1 {
				v = candidates[0]
				log.Infof("apply-keys: %s using sole vault for browser (dump path %q != local %q)",
					b.BrowserName(), v.UserDataDir, b.UserDataDir())
				found = true
			}
		}
		if !found {
			log.Warnf("apply-keys: %s no matching vault in dump", b.BrowserName())
			continue
		}
		km.SetRetrievers(masterkey.Retrievers{
			V10: maybeStaticRetriever(v.Keys.V10),
			V11: maybeStaticRetriever(v.Keys.V11),
			V20: maybeStaticRetriever(v.Keys.V20),
		})
	}
}

// maybeStaticRetriever wraps non-empty key bytes as a StaticRetriever; an empty/nil key returns nil
// to preserve the "tier not applicable" signal NewMasterKeys expects.
func maybeStaticRetriever(key []byte) masterkey.Retriever {
	if len(key) == 0 {
		return nil
	}
	return masterkey.NewStaticRetriever(key)
}
