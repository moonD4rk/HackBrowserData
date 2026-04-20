//go:build darwin

package browser

import (
	"fmt"
	"os"

	"github.com/moond4rk/keychainbreaker"
	"golang.org/x/term"

	"github.com/moond4rk/hackbrowserdata/crypto/keyretriever"
	"github.com/moond4rk/hackbrowserdata/log"
	"github.com/moond4rk/hackbrowserdata/types"
)

func platformBrowsers() []types.BrowserConfig {
	return []types.BrowserConfig{
		{
			Key:         "chrome",
			Name:        chromeName,
			Kind:        types.Chromium,
			Storage:     "Chrome",
			UserDataDir: homeDir + "/Library/Application Support/Google/Chrome",
		},
		{
			Key:         "edge",
			Name:        edgeName,
			Kind:        types.Chromium,
			Storage:     "Microsoft Edge",
			UserDataDir: homeDir + "/Library/Application Support/Microsoft Edge",
		},
		{
			Key:         "chromium",
			Name:        chromiumName,
			Kind:        types.Chromium,
			Storage:     "Chromium",
			UserDataDir: homeDir + "/Library/Application Support/Chromium",
		},
		{
			Key:         "chrome-beta",
			Name:        chromeBetaName,
			Kind:        types.Chromium,
			Storage:     "Chrome",
			UserDataDir: homeDir + "/Library/Application Support/Google/Chrome Beta",
		},
		{
			Key:         "opera",
			Name:        operaName,
			Kind:        types.ChromiumOpera,
			Storage:     "Opera",
			UserDataDir: homeDir + "/Library/Application Support/com.operasoftware.Opera",
		},
		{
			Key:         "opera-gx",
			Name:        operaGXName,
			Kind:        types.ChromiumOpera,
			Storage:     "Opera",
			UserDataDir: homeDir + "/Library/Application Support/com.operasoftware.OperaGX",
		},
		{
			Key:         "vivaldi",
			Name:        vivaldiName,
			Kind:        types.Chromium,
			Storage:     "Vivaldi",
			UserDataDir: homeDir + "/Library/Application Support/Vivaldi",
		},
		{
			Key:         "coccoc",
			Name:        coccocName,
			Kind:        types.Chromium,
			Storage:     "CocCoc",
			UserDataDir: homeDir + "/Library/Application Support/Coccoc",
		},
		{
			Key:         "brave",
			Name:        braveName,
			Kind:        types.Chromium,
			Storage:     "Brave",
			UserDataDir: homeDir + "/Library/Application Support/BraveSoftware/Brave-Browser",
		},
		{
			Key:         "yandex",
			Name:        yandexName,
			Kind:        types.ChromiumYandex,
			Storage:     "Yandex",
			UserDataDir: homeDir + "/Library/Application Support/Yandex/YandexBrowser",
		},
		{
			Key:         "arc",
			Name:        arcName,
			Kind:        types.Chromium,
			Storage:     "Arc",
			UserDataDir: homeDir + "/Library/Application Support/Arc/User Data",
		},
		{
			Key:         "firefox",
			Name:        firefoxName,
			Kind:        types.Firefox,
			UserDataDir: homeDir + "/Library/Application Support/Firefox/Profiles",
		},
		{
			Key:         "safari",
			Name:        safariName,
			Kind:        types.Safari,
			UserDataDir: homeDir + "/Library/Safari",
		},
	}
}

// resolveKeychainPassword returns the keychain password for macOS.
// If not provided via CLI flag, it prompts interactively when stdin is a TTY.
// After obtaining the password, it verifies against keychainbreaker; on any
// failure it returns "" so downstream code enters "no password" mode rather
// than propagating a known-bad credential. Safari then exports
// keychain-protected entries as metadata-only via keychainbreaker's partial
// extraction mode; Chromium falls back to SecurityCmdRetriever.
func resolveKeychainPassword(flagPassword string) string {
	password := flagPassword
	if password == "" {
		if !term.IsTerminal(int(os.Stdin.Fd())) {
			log.Warnf("macOS login password not provided and stdin is not a TTY; keychain-protected data will be exported as metadata only")
			return ""
		}
		fmt.Fprint(os.Stderr, "Enter macOS login password: ")
		pwd, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		if err != nil {
			log.Warnf("failed to read macOS login password: %v; keychain-protected data will be exported as metadata only", err)
			return ""
		}
		password = string(pwd)
	}

	if password == "" {
		log.Warnf("no macOS login password entered; keychain-protected data will be exported as metadata only")
		return ""
	}

	// Verify early: try to unlock keychain with keychainbreaker. On failure
	// return "" so KeychainPasswordRetriever and Safari both skip the credential
	// and rely on their respective fallback paths (SecurityCmdRetriever for
	// Chromium, metadata-only export for Safari).
	kc, err := keychainbreaker.Open()
	if err != nil {
		log.Warnf("keychain open failed: %v; keychain-protected data will be exported as metadata only", err)
		return ""
	}
	if err := kc.TryUnlock(keychainbreaker.WithPassword(password)); err != nil {
		log.Warnf("keychain unlock failed with provided password; keychain-protected data will be exported as metadata only")
		log.Debugf("keychain unlock detail: %v", err)
		return ""
	}

	return password
}

// keychainPasswordSetter is an optional capability interface satisfied by
// Safari, which reads InternetPassword records directly from the login keychain.
type keychainPasswordSetter interface {
	SetKeychainPassword(string)
}

// newPlatformInjector returns a closure that injects the Chromium master-key
// retriever and the Safari Keychain password into each Browser.
//
// Resolution is lazy: the keychain password prompt and retriever construction
// are deferred until the first Browser that actually needs them passes through
// the closure. Browsers that satisfy neither setter interface (e.g. Firefox)
// short-circuit without ever touching the keychain, so `-b firefox` on macOS
// no longer triggers a password prompt.
func newPlatformInjector(opts PickOptions) func(Browser) {
	var (
		password   string
		retrievers keyretriever.Retrievers
		resolved   bool
	)
	return func(b Browser) {
		rs, needsRetrievers := b.(keyRetrieversSetter)
		kps, needsKeychainPassword := b.(keychainPasswordSetter)
		if !needsRetrievers && !needsKeychainPassword {
			return
		}
		if !resolved {
			password = resolveKeychainPassword(opts.KeychainPassword)
			retrievers = keyretriever.DefaultRetrievers(password)
			resolved = true
		}
		if needsRetrievers {
			rs.SetKeyRetrievers(retrievers)
		}
		if needsKeychainPassword {
			kps.SetKeychainPassword(password)
		}
	}
}
