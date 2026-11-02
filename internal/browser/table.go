package browser

import "github.com/moond4rk/hackbrowserdata/internal/masterkey"

// browsers is in discovery order, which is also the order records reach the output files.
var browsers = []Browser{
	{
		Name: "chrome", DisplayName: "Chrome", Base: Chromium,
		Darwin: Platform{UserDataDirs: []string{"Library/Application Support/Google/Chrome"}, KeychainLabel: "Chrome"},
		Linux:  Platform{UserDataDirs: []string{".config/google-chrome"}, KeychainLabel: "Chrome Safe Storage"},
		Windows: Platform{
			UserDataDirs: []string{"AppData/Local/Google/Chrome/User Data"},
			Exe:          "chrome.exe",
			ExeFallbacks: []string{
				`%ProgramFiles%\Google\Chrome\Application\chrome.exe`,
				`%ProgramFiles(x86)%\Google\Chrome\Application\chrome.exe`,
				`%LocalAppData%\Google\Chrome\Application\chrome.exe`,
			},
			ABE: masterkey.ABEChromeBase,
		},
	},
	{
		Name: "edge", DisplayName: "Microsoft Edge", Base: Chromium,
		Darwin: Platform{UserDataDirs: []string{"Library/Application Support/Microsoft Edge"}, KeychainLabel: "Microsoft Edge"},
		Linux:  Platform{UserDataDirs: []string{".config/microsoft-edge"}, KeychainLabel: "Chromium Safe Storage"},
		Windows: Platform{
			UserDataDirs: []string{"AppData/Local/Microsoft/Edge/User Data"},
			Exe:          "msedge.exe",
			ExeFallbacks: []string{
				`%ProgramFiles(x86)%\Microsoft\Edge\Application\msedge.exe`,
				`%ProgramFiles%\Microsoft\Edge\Application\msedge.exe`,
			},
			ABE: masterkey.ABEEdge,
		},
	},
	{
		Name: "chromium", DisplayName: "Chromium", Base: Chromium,
		Darwin:  Platform{UserDataDirs: []string{"Library/Application Support/Chromium"}, KeychainLabel: "Chromium"},
		Linux:   Platform{UserDataDirs: []string{".config/chromium"}, KeychainLabel: "Chromium Safe Storage"},
		Windows: Platform{UserDataDirs: []string{"AppData/Local/Chromium/User Data"}},
	},
	{
		Name: "chrome-beta", DisplayName: "Chrome Beta", Base: Chromium,
		Darwin: Platform{UserDataDirs: []string{"Library/Application Support/Google/Chrome Beta"}, KeychainLabel: "Chrome"},
		Linux:  Platform{UserDataDirs: []string{".config/google-chrome-beta"}, KeychainLabel: "Chrome Safe Storage"},
		Windows: Platform{
			UserDataDirs: []string{"AppData/Local/Google/Chrome Beta/User Data"},
			Exe:          "chrome.exe",
			ExeFallbacks: []string{
				`%ProgramFiles%\Google\Chrome Beta\Application\chrome.exe`,
				`%ProgramFiles(x86)%\Google\Chrome Beta\Application\chrome.exe`,
				`%LocalAppData%\Google\Chrome Beta\Application\chrome.exe`,
			},
			ABE: masterkey.ABEChromeBase,
		},
	},
	{
		Name: "opera", DisplayName: "Opera", Base: Chromium,
		Darwin:  Platform{UserDataDirs: []string{"Library/Application Support/com.operasoftware.Opera"}, KeychainLabel: "Opera"},
		Linux:   Platform{UserDataDirs: []string{".config/opera"}, KeychainLabel: "Chromium Safe Storage"},
		Windows: Platform{UserDataDirs: []string{"AppData/Roaming/Opera Software/Opera Stable"}},
	},
	{
		Name: "opera-gx", DisplayName: "OperaGX", Base: Chromium,
		Darwin:  Platform{UserDataDirs: []string{"Library/Application Support/com.operasoftware.OperaGX"}, KeychainLabel: "Opera"},
		Windows: Platform{UserDataDirs: []string{"AppData/Roaming/Opera Software/Opera GX Stable"}},
	},
	{
		Name: "vought", DisplayName: "Browser from Vought", Base: Chromium,
		Windows: Platform{UserDataDirs: []string{"AppData/Roaming/Browser from Vought"}},
	},
	{
		Name: "vivaldi", DisplayName: "Vivaldi", Base: Chromium,
		Darwin:  Platform{UserDataDirs: []string{"Library/Application Support/Vivaldi"}, KeychainLabel: "Vivaldi"},
		Linux:   Platform{UserDataDirs: []string{".config/vivaldi"}, KeychainLabel: "Chrome Safe Storage"},
		Windows: Platform{UserDataDirs: []string{"AppData/Local/Vivaldi/User Data"}},
	},
	{
		Name: "coccoc", DisplayName: "CocCoc", Base: Chromium,
		Darwin: Platform{UserDataDirs: []string{"Library/Application Support/Coccoc"}, KeychainLabel: "CocCoc"},
		Windows: Platform{
			UserDataDirs: []string{"AppData/Local/CocCoc/Browser/User Data"},
			Exe:          "browser.exe",
			ExeFallbacks: []string{
				`%ProgramFiles%\CocCoc\Browser\Application\browser.exe`,
				`%ProgramFiles(x86)%\CocCoc\Browser\Application\browser.exe`,
				`%LocalAppData%\CocCoc\Browser\Application\browser.exe`,
			},
			ABE: masterkey.ABEChromeBase,
		},
	},
	{
		Name: "brave", DisplayName: "Brave", Base: Chromium,
		Darwin: Platform{UserDataDirs: []string{"Library/Application Support/BraveSoftware/Brave-Browser"}, KeychainLabel: "Brave"},
		Linux:  Platform{UserDataDirs: []string{".config/BraveSoftware/Brave-Browser"}, KeychainLabel: "Brave Safe Storage"},
		Windows: Platform{
			UserDataDirs: []string{"AppData/Local/BraveSoftware/Brave-Browser/User Data"},
			Exe:          "brave.exe",
			ExeFallbacks: []string{
				`%ProgramFiles%\BraveSoftware\Brave-Browser\Application\brave.exe`,
				`%ProgramFiles(x86)%\BraveSoftware\Brave-Browser\Application\brave.exe`,
				`%LocalAppData%\BraveSoftware\Brave-Browser\Application\brave.exe`,
			},
			ABE: masterkey.ABEChromeBase,
		},
	},
	{
		Name: "yandex", DisplayName: "Yandex", Base: Chromium,
		Darwin:  Platform{UserDataDirs: []string{"Library/Application Support/Yandex/YandexBrowser"}, KeychainLabel: "Yandex"},
		Windows: Platform{UserDataDirs: []string{"AppData/Local/Yandex/YandexBrowser/User Data"}},
	},
	{
		Name: "360x", DisplayName: "360 Speed X", Base: Chromium,
		Windows: Platform{UserDataDirs: []string{"AppData/Local/360ChromeX/Chrome/User Data"}},
	},
	{
		Name: "360", DisplayName: "360 Speed", Base: Chromium,
		Windows: Platform{UserDataDirs: []string{"AppData/Local/360chrome/Chrome/User Data"}},
	},
	{
		Name: "qq", DisplayName: "QQ", Base: Chromium,
		Windows: Platform{UserDataDirs: []string{"AppData/Local/Tencent/QQBrowser/User Data"}},
	},
	{
		Name: "dc", DisplayName: "DC", Base: Chromium,
		Windows: Platform{UserDataDirs: []string{"AppData/Local/DCBrowser/User Data"}},
	},
	{
		Name: "sogou", DisplayName: "Sogou", Base: Chromium,
		Windows: Platform{UserDataDirs: []string{"AppData/Local/Sogou/SogouExplorer/User Data"}},
	},
	{
		Name: "arc", DisplayName: "Arc", Base: Chromium,
		Darwin:  Platform{UserDataDirs: []string{"Library/Application Support/Arc/User Data"}, KeychainLabel: "Arc"},
		Windows: Platform{UserDataDirs: []string{"AppData/Local/Packages/TheBrowserCompany.Arc_*/LocalCache/Local/Arc/User Data"}},
	},
	{
		Name: "duckduckgo", DisplayName: "DuckDuckGo", Base: Chromium,
		Windows: Platform{UserDataDirs: []string{"AppData/Local/Packages/DuckDuckGo.DesktopBrowser_*/LocalState/EBWebView"}},
	},
	{
		Name: "firefox", DisplayName: "Firefox", Base: Firefox,
		Darwin:  Platform{UserDataDirs: []string{"Library/Application Support/Firefox/Profiles"}},
		Linux:   Platform{UserDataDirs: []string{".mozilla/firefox"}},
		Windows: Platform{UserDataDirs: []string{"AppData/Roaming/Mozilla/Firefox/Profiles"}},
	},
	{
		Name: "safari", DisplayName: "Safari", Base: Safari,
		Darwin: Platform{UserDataDirs: []string{"Library/Safari"}},
	},
}
