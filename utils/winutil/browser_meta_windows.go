//go:build windows

// Package winutil provides high-level Windows utilities for HackBrowserData,
// built on the low-level syscall wrappers in utils/winapi.
//
// It currently covers:
//   - Browser executable resolution via registry App Paths + install-path
//     fallbacks (browser_path_windows.go).
//   - A single source of truth for Windows-side browser metadata: executable
//     name, install fallbacks, and ABE dispatch kind (browser_meta_windows.go).
//
// The C-side counterpart — CLSID / IID / vtable-slot bytes consumed by the
// reflective payload — lives in crypto/windows/abe_native/com_iid.c and
// must stay separate: the payload runs inside the injected browser process
// with no Go runtime.
package winutil

// ABEKind selects the App-Bound Encryption dispatch path used by the injected
// payload for this browser. DPAPI-only browsers (classic v10/v11) use ABENone;
// v20-capable Chromium forks pick a vtable slot based on which IElevator
// flavor their elevation_service exposes.
type ABEKind int

const (
	// ABENone means this browser has no ABE path — the key retriever chain
	// falls through to DPAPI for v10/v11.
	ABENone ABEKind = iota
	// ABEChromeBase is IElevator slot 5 (Chrome, Brave, CocCoc).
	ABEChromeBase
	// ABEEdge is IElevator slot 8 (Edge; prepends 3 extra interface methods).
	ABEEdge
	// ABEAvast is IElevator slot 13 (Avast; extended IElevator).
	ABEAvast
)

// Entry is the per-browser Windows metadata record.
//
// Key must match browser.BrowserConfig.Key for every config that sets
// WindowsABE: true, so retrievers and path resolvers share a single lookup
// identifier. CLSID/IID bytes are *not* stored here; see the package doc for why.
type Entry struct {
	Key              string
	ExeName          string
	InstallFallbacks []string
	ABE              ABEKind
}

// Table is the authoritative Go-side map of Windows browser metadata.
// Adding a new Chromium fork on the Go side is a single-entry edit here.
// The corresponding C-side CLSID/IID table lives in com_iid.c.
var Table = map[string]Entry{
	"chrome": {
		Key:     "chrome",
		ExeName: "chrome.exe",
		InstallFallbacks: []string{
			`%ProgramFiles%\Google\Chrome\Application\chrome.exe`,
			`%ProgramFiles(x86)%\Google\Chrome\Application\chrome.exe`,
			`%LocalAppData%\Google\Chrome\Application\chrome.exe`,
		},
		ABE: ABEChromeBase,
	},
	"chrome-beta": {
		Key:     "chrome-beta",
		ExeName: "chrome.exe",
		InstallFallbacks: []string{
			`%ProgramFiles%\Google\Chrome Beta\Application\chrome.exe`,
			`%ProgramFiles(x86)%\Google\Chrome Beta\Application\chrome.exe`,
			`%LocalAppData%\Google\Chrome Beta\Application\chrome.exe`,
		},
		ABE: ABEChromeBase,
	},
	"edge": {
		Key:     "edge",
		ExeName: "msedge.exe",
		InstallFallbacks: []string{
			`%ProgramFiles(x86)%\Microsoft\Edge\Application\msedge.exe`,
			`%ProgramFiles%\Microsoft\Edge\Application\msedge.exe`,
		},
		ABE: ABEEdge,
	},
	"brave": {
		Key:     "brave",
		ExeName: "brave.exe",
		InstallFallbacks: []string{
			`%ProgramFiles%\BraveSoftware\Brave-Browser\Application\brave.exe`,
			`%ProgramFiles(x86)%\BraveSoftware\Brave-Browser\Application\brave.exe`,
			`%LocalAppData%\BraveSoftware\Brave-Browser\Application\brave.exe`,
		},
		ABE: ABEChromeBase,
	},
	"coccoc": {
		Key:     "coccoc",
		ExeName: "browser.exe",
		InstallFallbacks: []string{
			`%ProgramFiles%\CocCoc\Browser\Application\browser.exe`,
			`%ProgramFiles(x86)%\CocCoc\Browser\Application\browser.exe`,
			`%LocalAppData%\CocCoc\Browser\Application\browser.exe`,
		},
		ABE: ABEChromeBase,
	},
}
