package firefox

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

func TestQueryMetaData(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	rows := sqlmock.NewRows([]string{"item1", "item2"}).
		AddRow([]byte("globalSalt"), []byte("metaBytes"))
	mock.ExpectQuery("SELECT item1, item2 FROM metaData WHERE id = 'password'").WillReturnRows(rows)

	globalSalt, metaBytes, err := queryMetaData(db)
	assert.NoError(t, err)
	assert.Equal(t, []byte("globalSalt"), globalSalt)
	assert.Equal(t, []byte("metaBytes"), metaBytes)
}

func TestQueryNssPrivate(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	rows := sqlmock.NewRows([]string{"a11", "a102"}).
		AddRow([]byte("nssA11"), []byte("nssA102"))
	mock.ExpectQuery("SELECT a11, a102 from nssPrivate").WillReturnRows(rows)

	nssA11, nssA102, err := queryNssPrivate(db)
	assert.NoError(t, err)
	assert.Equal(t, []byte("nssA11"), nssA11)
	assert.Equal(t, []byte("nssA102"), nssA102)
}
