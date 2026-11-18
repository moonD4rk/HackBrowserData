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

func TestSaltWithPassword(t *testing.T) {
	salt := []byte("saltsalt")

	// An empty password derives from the global salt alone.
	assert.Equal(t, salt, saltWithPassword(salt, nil))

	// A password is appended after the salt.
	assert.Equal(t, []byte("saltsaltpw"), saltWithPassword(salt, []byte("pw")))

	// The result must not share globalSalt's backing array, which the password check and every key entry reuse.
	got := saltWithPassword(salt, []byte("pw"))
	got[0] = 'X'
	assert.Equal(t, []byte("saltsalt"), salt, "globalSalt must not be mutated")
}

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
	cipher, err := pbe.Encrypt(saltWithPassword(globalSalt, password), []byte("password-check"))
	require.NoError(t, err)
	require.Len(t, cipher, 16, "a one-block ciphertext replaces the vector's own")
	return append(append([]byte(nil), blob[:len(blob)-len(cipher)]...), cipher...)
}

func constPassword(password string) func() ([]byte, error) {
	return func() ([]byte, error) {
		if password == "" {
			return nil, nil
		}
		return []byte(password), nil
	}
}

func TestSelectPassword_PrimaryPassword(t *testing.T) {
	globalSalt := bytes.Repeat([]byte("moond4rk"), 3)
	const password = "hunter2"
	key4 := &key4DB{globalSalt: globalSalt, passwordCheck: passwordCheckFor(t, globalSalt, []byte(password))}

	selected, err := key4.selectPassword(constPassword(password))
	require.NoError(t, err)
	assert.Equal(t, []byte(password), selected, "the right primary password is selected")

	_, err = key4.selectPassword(constPassword(""))
	require.ErrorIs(t, err, errPrimaryPasswordRequired, "a protected profile with no password asks for one")

	_, err = key4.selectPassword(constPassword("wrong"))
	require.ErrorIs(t, err, errPrimaryPasswordWrong, "a wrong password is reported as wrong")
}

func TestSelectPassword_NoPrimaryPassword(t *testing.T) {
	globalSalt := bytes.Repeat([]byte("moond4rk"), 3)
	key4 := &key4DB{globalSalt: globalSalt, passwordCheck: passwordCheckFor(t, globalSalt, nil)}

	selected, err := key4.selectPassword(func() ([]byte, error) {
		t.Fatal("must not read the user's password when the empty one fits")
		return nil, nil
	})
	require.NoError(t, err)
	assert.Nil(t, selected, "the empty password is used")
}

func TestSelectPassword_CorruptCheck(t *testing.T) {
	// A password check that will not parse is a corrupt DB, not a wrong or missing password.
	key4 := &key4DB{globalSalt: []byte("salt"), passwordCheck: []byte("not a valid pbe blob")}
	_, err := key4.selectPassword(constPassword("whatever"))
	require.Error(t, err)
	require.NotErrorIs(t, err, errPrimaryPasswordRequired)
	require.NotErrorIs(t, err, errPrimaryPasswordWrong)
}

func TestRetrieveMasterKey_PrimaryPassword(t *testing.T) {
	dir := t.TempDir()
	globalSalt := []byte("home-global-salt-20b")
	password := []byte("hunter2")
	masterKey := []byte("firefox-pw-master-key-24")
	browsertest.WriteFirefoxPrimaryPasswordProfile(t, dir, globalSalt, password, [][]byte{masterKey})
	key4, logins := filepath.Join(dir, "key4.db"), filepath.Join(dir, "logins.json")

	got, err := retrieveMasterKey(key4, logins, constPassword(string(password)))
	require.NoError(t, err)
	assert.Equal(t, masterKey, got, "the right password recovers the master key")

	_, err = retrieveMasterKey(key4, logins, constPassword(""))
	require.ErrorIs(t, err, errPrimaryPasswordRequired, "a protected profile with no password asks for one")

	_, err = retrieveMasterKey(key4, logins, constPassword("wrong"))
	require.ErrorIs(t, err, errPrimaryPasswordWrong, "a wrong password is reported as wrong")
}
