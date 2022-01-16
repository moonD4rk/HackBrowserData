package browser

import (
	"bytes"
	"crypto/sha1"
	"errors"
	"os/exec"

	"golang.org/x/crypto/pbkdf2"
)

var (
	chromiumList = map[string]struct {
		browserInfo *browserInfo
		items       []item
		// New         func(browser *browserInfo, items []item) *firefox
	}{
		"chrome": {
			browserInfo: chromeInfo,
			items:       defaultChromiumItems,
		},
		"edge": {
			browserInfo: edgeInfo,
			items:       defaultChromiumItems,
		},
	}
	firefoxList = map[string]struct {
		browserInfo *browserInfo
		items       []item
	}{
		"firefox": {
			browserInfo: firefoxInfo,
			items:       defaultFirefoxItems,
		},
	}
)

var (
	ErrWrongSecurityCommand = errors.New("macOS wrong security command")
)

func (c *chromium) GetMasterKey() ([]byte, error) {
	var (
		cmd            *exec.Cmd
		stdout, stderr bytes.Buffer
	)
	// $ security find-generic-password -wa 'Chrome'
	cmd = exec.Command("security", "find-generic-password", "-wa", c.browserInfo.storage)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return nil, err
	}
	if stderr.Len() > 0 {
		return nil, errors.New(stderr.String())
	}
	temp := stdout.Bytes()
	chromeSecret := temp[:len(temp)-1]
	if chromeSecret == nil {
		return nil, ErrWrongSecurityCommand
	}
	var chromeSalt = []byte("saltysalt")
	// @https://source.chromium.org/chromium/chromium/src/+/master:components/os_crypt/os_crypt_mac.mm;l=157
	key := pbkdf2.Key(chromeSecret, chromeSalt, 1003, 16, sha1.New)
	c.browserInfo.masterKey = key
	return key, nil
}

var (
	chromeInfo = &browserInfo{
		name:        chromeName,
		storage:     chromeStorageName,
		profilePath: chromeProfilePath,
	}
	edgeInfo = &browserInfo{
		name:        edgeName,
		storage:     edgeStorageName,
		profilePath: edgeProfilePath,
	}
	firefoxInfo = &browserInfo{
		name:        firefoxName,
		profilePath: firefoxProfilePath,
	}
)

const (
	chromeProfilePath     = "/Library/Application Support/Google/Chrome/"
	chromeBetaProfilePath = "/Library/Application Support/Google/Chrome Beta/"
	chromiumProfilePath   = "/Library/Application Support/Chromium/"
	edgeProfilePath       = "/Library/Application Support/Microsoft Edge/"
	braveProfilePath      = "/Library/Application Support/BraveSoftware/Brave-Browser/"
	operaProfilePath      = "/Library/Application Support/com.operasoftware.Opera/"
	operaGXProfilePath    = "/Library/Application Support/com.operasoftware.OperaGX/"
	vivaldiProfilePath    = "/Library/Application Support/Vivaldi/"
	coccocProfilePath     = "/Library/Application Support/Coccoc/"
	yandexProfilePath     = "/Library/Application Support/Yandex/YandexBrowser/"

	firefoxProfilePath = "/Library/Application Support/Firefox/Profiles/"
)

const (
	chromeStorageName     = "Chrome"
	chromeBetaStorageName = "Chrome"
	chromiumStorageName   = "Chromium"
	edgeStorageName       = "Microsoft Edge"
	braveStorageName      = "Brave"
	operaStorageName      = "Opera"
	vivaldiStorageName    = "Vivaldi"
	coccocStorageName     = "CocCoc"
	yandexStorageName     = "Yandex"
)
