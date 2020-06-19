package utils

const (
	winChromeDir = "/Users/*/Library/Application Support/Google/Chrome/*/"
)

func GetDBPath(dbName string) string {
	s, err := filepath.Glob(winChromeDir + dbName)
	if err != nil && len(s) == 0 {
		panic(err)
	}
	return s[0]
}

func AesGCMDecrypt(crypted, key, nounce []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	blockMode, _ := cipher.NewGCM(block)
	origData, err := blockMode.Open(nil, nounce, crypted, nil)
	if err != nil{
		return nil, err
	}
	return origData, nil
}
