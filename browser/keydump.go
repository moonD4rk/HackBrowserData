package browser

import (
	"runtime"

	"github.com/moond4rk/hackbrowserdata/crypto/keyretriever"
	"github.com/moond4rk/hackbrowserdata/log"
)

// BuildDump exports per-installation master keys. Each Browser is one installation,
// so this is a straight one-Vault-per-installation map: ExportKeys is invoked once
// per installation. Installations without KeyManager (Firefox/Safari) are skipped.
// Partial results (e.g. V10 retrieved, V20 failed) keep the usable tiers rather than
// discarding the vault — a Chrome 127+ profile mixes v10 + v20 ciphertexts and a
// v20-only failure must not erase a usable v10 key.
func BuildDump(browsers []Browser) keyretriever.Dump {
	dump := keyretriever.NewDump()
	for _, b := range browsers {
		km, ok := b.(KeyManager)
		if !ok {
			continue
		}
		keys, err := km.ExportKeys()
		if err != nil {
			status := "partial"
			if !keys.HasAny() {
				status = "failed"
			}
			log.Warnf("dump-keys: %s %s: %v", b.BrowserName(), status, err)
		}
		if !keys.HasAny() {
			continue
		}
		dump.Vaults = append(dump.Vaults, keyretriever.Vault{
			Browser:     b.BrowserName(),
			UserDataDir: b.UserDataDir(),
			Profiles:    profileNames(b),
			Keys:        keys,
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

// ApplyDump installs master keys from dump onto matching installations, replacing
// each installation's default platform-native retrievers with StaticProviders
// backed by the Dump's bytes. Matching is by (BrowserName, UserDataDir) — the same
// key BuildDump emits. When exact match fails (commonly a cross-host path mismatch:
// Windows backslash vs POSIX, or a relocated User Data dir via -p), falls back to
// the sole vault for that browser name when one exists. Installations without a
// matching vault are warned and left untouched; non-KeyManager installations
// (Firefox/Safari) are skipped silently.
func ApplyDump(browsers []Browser, dump keyretriever.Dump) {
	if dump.Host.OS != "" && dump.Host.OS != runtime.GOOS {
		log.Infof("apply-keys: dump created on %s/%s; current host is %s/%s",
			dump.Host.OS, dump.Host.Arch, runtime.GOOS, runtime.GOARCH)
	}
	vaultIndex := make(map[string]*keyretriever.Vault, len(dump.Vaults))
	vaultsByBrowser := make(map[string][]*keyretriever.Vault)
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
		km.SetKeyRetrievers(keyretriever.Retrievers{
			V10: maybeStaticProvider(v.Keys.V10),
			V11: maybeStaticProvider(v.Keys.V11),
			V20: maybeStaticProvider(v.Keys.V20),
		})
	}
}

// maybeStaticProvider wraps non-empty key bytes as a StaticProvider; an empty/nil key returns nil
// to preserve the "tier not applicable" signal NewMasterKeys expects.
func maybeStaticProvider(key []byte) keyretriever.KeyRetriever {
	if len(key) == 0 {
		return nil
	}
	return keyretriever.NewStaticProvider(key)
}
