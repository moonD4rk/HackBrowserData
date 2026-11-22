package firefox

import (
	"bytes"
	"encoding/hex"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/internal/browser/browsertest"
	"github.com/moond4rk/hackbrowserdata/internal/crypto"
)

// passwordCheckPBEHex is a real metaData.item2 PBE blob from crypto's vectors (global salt "moond4rk" x3); its single
// 16-byte ciphertext block is swapped for one over "password-check" to make a primary-password fixture.
const passwordCheckPBEHex = "307a3066060960864801650304012e3059303a060960864801650304012e302d04186d6f6f6e6434726b6d6f6f6e6434726b6d6f6f6e6434726b020101020120300b060960864801650304012e301b060960864801650304012e040e303132333435363730313233343504100474679f2e6256518b7adb877beaa154"

// passwordCheckFor builds a metaData.item2 that decrypts to "password-check" only under globalSalt + password.
func passwordCheckFor(t *testing.T, globalSalt, password []byte) []byte {
	t.Helper()
	blob, err := hex.DecodeString(passwordCheckPBEHex)
	require.NoError(t, err)
	pbe, err := crypto.NewASN1PBE(blob)
	require.NoError(t, err)
	cipher, err := pbe.Encrypt(crypto.NSSPasswordKey(globalSalt, password), []byte("password-check"))
	require.NoError(t, err)
	require.Len(t, cipher, 16, "a one-block ciphertext replaces the vector's own")
	return append(append([]byte(nil), blob[:len(blob)-len(cipher)]...), cipher...)
}

func TestSelectPassword_PrimaryPassword(t *testing.T) {
	globalSalt := bytes.Repeat([]byte("moond4rk"), 3)
	const password = "hunter2"
	key4 := &key4DB{globalSalt: globalSalt, passwordCheck: passwordCheckFor(t, globalSalt, []byte(password))}

	selected, err := key4.selectPassword([]byte(password))
	require.NoError(t, err)
	assert.Equal(t, []byte(password), selected, "the right primary password is selected")

	_, err = key4.selectPassword(nil)
	require.ErrorIs(t, err, errPrimaryPasswordRequired, "a protected profile with no password asks for one")

	_, err = key4.selectPassword([]byte("wrong"))
	require.ErrorIs(t, err, errPrimaryPasswordWrong, "a wrong password is reported as wrong")
}

func TestSelectPassword_NoPrimaryPassword(t *testing.T) {
	globalSalt := bytes.Repeat([]byte("moond4rk"), 3)
	key4 := &key4DB{globalSalt: globalSalt, passwordCheck: passwordCheckFor(t, globalSalt, nil)}

	selected, err := key4.selectPassword([]byte("hunter2"))
	require.NoError(t, err)
	assert.Nil(t, selected, "the empty password fits, so a given one is not used")
}

func TestSelectPassword_CorruptCheck(t *testing.T) {
	// A password check that will not parse is a corrupt DB, not a wrong or missing password.
	key4 := &key4DB{globalSalt: []byte("salt"), passwordCheck: []byte("not a valid pbe blob")}
	_, err := key4.selectPassword([]byte("whatever"))
	require.Error(t, err)
	require.NotErrorIs(t, err, errPrimaryPasswordRequired)
	require.NotErrorIs(t, err, errPrimaryPasswordWrong)
}

func TestRetrieveMasterKey_PrimaryPassword(t *testing.T) {
	dir := t.TempDir()
	globalSalt := bytes.Repeat([]byte("16-byte-salt-abc"), 3) // NSS's 48-byte salt for a newly set primary password
	password := []byte("hunter2")
	masterKey := []byte("firefox-pw-master-key-24")
	browsertest.WriteFirefoxPrimaryPasswordProfile(t, dir, globalSalt, password, [][]byte{masterKey})
	key4, logins := filepath.Join(dir, "key4.db"), filepath.Join(dir, "logins.json")

	got, err := retrieveMasterKey(key4, logins, password)
	require.NoError(t, err)
	assert.Equal(t, masterKey, got, "the right password recovers the master key")

	_, err = retrieveMasterKey(key4, logins, nil)
	require.ErrorIs(t, err, errPrimaryPasswordRequired, "a protected profile with no password asks for one")

	_, err = retrieveMasterKey(key4, logins, []byte("wrong"))
	require.ErrorIs(t, err, errPrimaryPasswordWrong, "a wrong password is reported as wrong")
}
