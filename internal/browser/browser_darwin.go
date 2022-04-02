package browser

import (
	"bytes"
	"crypto/sha1"
	"errors"
	"os/exec"

	"golang.org/x/crypto/pbkdf2"

	"hack-browser-data/internal/item"
)

var (
	chromiumList = map[string]struct {
		browserInfo *browserInfo
		items       []item.Item
	}{
		"chrome": {
			browserInfo: chromeInfo,
			items:       item.DefaultChromium,
		},
		"edge": {
			browserInfo: edgeInfo,
			items:       item.DefaultChromium,
		},
		"chromium": {
			browserInfo: chromiumInfo,
			items:       item.DefaultChromium,
		},
		"chrome-beta": {
			browserInfo: chromeBetaInfo,
			items:       item.DefaultChromium,
		},
		"opera": {
			browserInfo: operaInfo,
			items:       item.DefaultChromium,
		},
		"opera-gx": {
			browserInfo: operaGXInfo,
			items:       item.DefaultChromium,
		},
		"vivaldi": {
			browserInfo: vivaldiInfo,
			items:       item.DefaultChromium,
		},
		"coccoc": {
			browserInfo: coccocInfo,
			items:       item.DefaultChromium,
		},
		"brave": {
			browserInfo: braveInfo,
			items:       item.DefaultChromium,
		},
		"yandex": {
			browserInfo: yandexInfo,
			items:       item.DefaultYandex,
		},
	}
	firefoxList = map[string]struct {
		browserInfo *browserInfo
		items       []item.Item
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
	if key != nil {
		c.browserInfo.masterKey = key
		return key, nil
	}
	return nil, errors.New("macOS wrong security command")
}

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

var (
	chromeInfo = &browserInfo{
		name:        chromeName,
		storage:     chromeStorageName,
		profilePath: chromeProfilePath,
	}
	chromiumInfo = &browserInfo{
		name:        chromiumName,
		storage:     chromiumStorageName,
		profilePath: chromiumProfilePath,
	}
	chromeBetaInfo = &browserInfo{
		name:        chromeBetaName,
		storage:     chromeBetaStorageName,
		profilePath: chromeBetaProfilePath,
	}
	operaInfo = &browserInfo{
		name:        operaName,
		profilePath: operaProfilePath,
		storage:     operaStorageName,
	}
	operaGXInfo = &browserInfo{
		name:        operaGXName,
		profilePath: operaGXProfilePath,
		storage:     operaStorageName,
	}
	edgeInfo = &browserInfo{
		name:        edgeName,
		storage:     edgeStorageName,
		profilePath: edgeProfilePath,
	}
	braveInfo = &browserInfo{
		name:        braveName,
		profilePath: braveProfilePath,
		storage:     braveStorageName,
	}
	vivaldiInfo = &browserInfo{
		name:        vivaldiName,
		storage:     vivaldiStorageName,
		profilePath: vivaldiProfilePath,
	}
	coccocInfo = &browserInfo{
		name:        coccocName,
		storage:     coccocStorageName,
		profilePath: coccocProfilePath,
	}
	yandexInfo = &browserInfo{
		name:        yandexName,
		storage:     yandexStorageName,
		profilePath: yandexProfilePath,
	}
	firefoxInfo = &browserInfo{
		name:        firefoxName,
		profilePath: firefoxProfilePath,
	}
)
