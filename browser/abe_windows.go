//go:build windows

package browser

import "github.com/moond4rk/hackbrowserdata/internal/masterkey"

// abeTarget is a Windows ABE browser's executable and elevation-service dialect, keyed by BrowserConfig.Key.
// It stays beside the browser table until browser.Def.Platform absorbs it.
type abeTarget struct {
	exe       string
	fallbacks []string
	kind      masterkey.ABEKind
}

var abeTargets = map[string]abeTarget{
	"chrome": {
		exe: "chrome.exe",
		fallbacks: []string{
			`%ProgramFiles%\Google\Chrome\Application\chrome.exe`,
			`%ProgramFiles(x86)%\Google\Chrome\Application\chrome.exe`,
			`%LocalAppData%\Google\Chrome\Application\chrome.exe`,
		},
		kind: masterkey.ABEChromeBase,
	},
	"chrome-beta": {
		exe: "chrome.exe",
		fallbacks: []string{
			`%ProgramFiles%\Google\Chrome Beta\Application\chrome.exe`,
			`%ProgramFiles(x86)%\Google\Chrome Beta\Application\chrome.exe`,
			`%LocalAppData%\Google\Chrome Beta\Application\chrome.exe`,
		},
		kind: masterkey.ABEChromeBase,
	},
	"edge": {
		exe: "msedge.exe",
		fallbacks: []string{
			`%ProgramFiles(x86)%\Microsoft\Edge\Application\msedge.exe`,
			`%ProgramFiles%\Microsoft\Edge\Application\msedge.exe`,
		},
		kind: masterkey.ABEEdge,
	},
	"brave": {
		exe: "brave.exe",
		fallbacks: []string{
			`%ProgramFiles%\BraveSoftware\Brave-Browser\Application\brave.exe`,
			`%ProgramFiles(x86)%\BraveSoftware\Brave-Browser\Application\brave.exe`,
			`%LocalAppData%\BraveSoftware\Brave-Browser\Application\brave.exe`,
		},
		kind: masterkey.ABEChromeBase,
	},
	"coccoc": {
		exe: "browser.exe",
		fallbacks: []string{
			`%ProgramFiles%\CocCoc\Browser\Application\browser.exe`,
			`%ProgramFiles(x86)%\CocCoc\Browser\Application\browser.exe`,
			`%LocalAppData%\CocCoc\Browser\Application\browser.exe`,
		},
		kind: masterkey.ABEChromeBase,
	},
}
