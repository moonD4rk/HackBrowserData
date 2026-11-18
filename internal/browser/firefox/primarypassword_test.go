package firefox

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestSelectPassword_NoValidCheck(t *testing.T) {
	// A password check that decrypts for no password looks like a profile whose password is simply wrong or missing.
	key4 := &key4DB{globalSalt: []byte("salt"), passwordCheck: []byte("not a valid pbe blob")}

	_, err := key4.selectPassword(nil)
	require.ErrorIs(t, err, errPrimaryPasswordRequired)

	_, err = key4.selectPassword([]byte("hunter2"))
	require.ErrorIs(t, err, errPrimaryPasswordWrong)
}
