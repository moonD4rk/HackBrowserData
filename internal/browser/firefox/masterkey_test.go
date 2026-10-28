package firefox

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadKey4DB(t *testing.T) {
	// Create a minimal key4.db with metaData and nssPrivate tables
	path := createTestDB(t, "key4.db",
		[]string{
			`CREATE TABLE metaData (id TEXT PRIMARY KEY, item1 BLOB, item2 BLOB)`,
			`CREATE TABLE nssPrivate (a11 BLOB, a102 BLOB)`,
		},
		`INSERT INTO metaData (id, item1, item2) VALUES ('password', x'aabbccdd', x'11223344')`,
		`INSERT INTO nssPrivate (a11, a102) VALUES (x'deadbeef', x'cafebabe')`,
		`INSERT INTO nssPrivate (a11, a102) VALUES (x'feedface', x'12345678')`,
	)

	k4, err := readKey4DB(path)
	require.NoError(t, err)

	assert.Equal(t, []byte{0xaa, 0xbb, 0xcc, 0xdd}, k4.globalSalt)
	assert.Equal(t, []byte{0x11, 0x22, 0x33, 0x44}, k4.passwordCheck)
	require.Len(t, k4.privateKeys, 2)
	// Don't assume row order — check that both entries exist
	encryptedBlobs := map[string]bool{}
	for _, pk := range k4.privateKeys {
		encryptedBlobs[fmt.Sprintf("%x", pk.encrypted)] = true
	}
	assert.True(t, encryptedBlobs["deadbeef"])
	assert.True(t, encryptedBlobs["feedface"])
}

func TestReadKey4DB_EmptyNssPrivate(t *testing.T) {
	path := createTestDB(t, "key4.db",
		[]string{
			`CREATE TABLE metaData (id TEXT PRIMARY KEY, item1 BLOB, item2 BLOB)`,
			`CREATE TABLE nssPrivate (a11 BLOB, a102 BLOB)`,
		},
		`INSERT INTO metaData (id, item1, item2) VALUES ('password', x'aa', x'bb')`,
	)

	_, err := readKey4DB(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

func TestSampleEncryptedLogins(t *testing.T) {
	raw := []byte(`{"logins":[
		{"encryptedUsername":"dGVzdA==","encryptedPassword":"cGFzcw=="},
		{"encryptedUsername":"!!!invalid","encryptedPassword":"cGFzcw=="},
		{"encryptedUsername":"dGVzdA==","encryptedPassword":"cGFzcw=="}
	]}`)

	samples := sampleEncryptedLogins(raw)
	require.Len(t, samples, 2) // second entry skipped (invalid base64)
	assert.Equal(t, []byte("test"), samples[0].username)
	assert.Equal(t, []byte("pass"), samples[0].password)
}
