package browser

import (
	"github.com/moond4rk/hackbrowserdata/crypto/keyretriever"
	"github.com/moond4rk/hackbrowserdata/log"
)

// BuildDump assembles a cross-host Dump from a set of fully-wired browsers.
// Browsers that do not implement KeyManager (Firefox, Safari) are skipped —
// their master keys aren't portable across hosts. Profiles sharing the same
// (BrowserName, UserDataDir) are grouped into one Installation and ExportKeys
// runs only once per group, matching the per-installation nature of Chromium's
// master keys.
//
// Per-installation ExportKeys failures are logged and the installation is
// dropped; the rest of the dump proceeds. An empty browser list returns a
// Dump with metadata but no Installations.
func BuildDump(browsers []Browser) keyretriever.Dump {
	dump := keyretriever.NewDump()

	type instKey struct {
		browser     string
		userDataDir string
	}
	seen := make(map[instKey]int)

	for _, b := range browsers {
		km, ok := b.(KeyManager)
		if !ok {
			log.Debugf("dump-keys: %s skipped (not key-portable)", b.BrowserName())
			continue
		}

		key := instKey{b.BrowserName(), b.UserDataDir()}
		if idx, exists := seen[key]; exists {
			dump.Installations[idx].Profiles = append(dump.Installations[idx].Profiles, b.ProfileName())
			continue
		}

		keys, err := km.ExportKeys()
		if err != nil {
			log.Warnf("dump-keys: %s/%s export failed: %v", b.BrowserName(), b.ProfileName(), err)
			continue
		}

		seen[key] = len(dump.Installations)
		dump.Installations = append(dump.Installations, keyretriever.Installation{
			Browser:     b.BrowserName(),
			UserDataDir: b.UserDataDir(),
			Profiles:    []string{b.ProfileName()},
			Keys: keyretriever.InstallKeys{
				V10: keys.V10,
				V11: keys.V11,
				V20: keys.V20,
			},
		})
	}

	return dump
}
