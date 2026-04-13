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
// After obtaining the password, it verifies against keychainbreaker and warns
// early if decryption will fail (e.g. on newer macOS versions).
func resolveKeychainPassword(flagPassword string) string {
	password := flagPassword
	if password == "" {
		if !term.IsTerminal(int(os.Stdin.Fd())) {
			return ""
		}
		fmt.Fprint(os.Stderr, "Enter macOS login password: ")
		pwd, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		if err != nil {
			log.Debugf("read password: %v", err)
			return ""
		}
		password = string(pwd)
	}

	// Verify early: try to unlock keychain with keychainbreaker.
	// If it fails, Chromium can still fall back to SecurityCmdRetriever,
	// but Safari passwords will be empty (metadata only).
	if password != "" {
		kc, err := keychainbreaker.Open()
		if err != nil {
			log.Warnf("keychain open failed: %v", err)
		} else if err := kc.TryUnlock(keychainbreaker.WithPassword(password)); err != nil {
			log.Warnf("keychain unlock failed with provided password")
			log.Debugf("keychain unlock detail: %v", err)
		}
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
func newPlatformInjector(opts PickOptions) func(Browser) {
	password := resolveKeychainPassword(opts.KeychainPassword)
	retriever := keyretriever.DefaultRetriever(password)
	return func(b Browser) {
		if s, ok := b.(retrieverSetter); ok {
			s.SetRetriever(retriever)
		}
		if s, ok := b.(keychainPasswordSetter); ok {
			s.SetKeychainPassword(password)
		}
	}
}
