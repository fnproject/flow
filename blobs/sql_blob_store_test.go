package blobs

import (
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"bytes"
	"os"
	"fmt"
	"path"
	"database/sql"
)

func TestShouldInsertBlobAndGenerateId(t *testing.T) {
	store := givenEmptyBlobStore()

	data := []byte("Some data")
	blob, err := store.Create("graph", "test/type", bytes.NewReader(data))

	assert.NoError(t, err)
	require.NotNil(t, blob)
	assert.NotNil(t, blob.ID)
	assert.Equal(t, "test/type", blob.ContentType)
	assert.Equal(t, uint64(len(data)), blob.Length)

}

func TestShouldRetrieveStoredBlob(t *testing.T) {
	store := givenEmptyBlobStore()

	data := []byte("Some data")
	blob, err := store.Create("graph", "test/type", bytes.NewReader(data))
	require.NoError(t, err)

	newData, err := store.Read("graph", blob.ID)
	assert.NoError(t, err)
	assert.Equal(t, data, newData)

}

func TestShouldFailWithUnknownBlob(t *testing.T) {
	store := givenEmptyBlobStore()

	newData, err := store.Read("graph", "foo")
	assert.Nil(t, newData)
	assert.Error(t, err)

}
func TestShouldReadAndWriteEmptyBlob(t *testing.T) {
	store := givenEmptyBlobStore()

	blob, err := store.Create("graph", "test/type", bytes.NewReader([]byte{}))
	require.NoError(t, err)
	assert.Equal(t, uint64(0), blob.Length)

	data, err := store.Read("graph", blob.ID)
	assert.NoError(t, err)
	assert.Empty(t, data)
}

func givenEmptyBlobStore() Store {

	db := setupDb()
	store, err := NewSQLBlobStore(db)
	if err != nil {
		panic(err)
	}
	return store
}

func setupDb() *sqlx.DB {
	os.RemoveAll(dbPath)

	dir := dbPath
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		panic(err)
	}

	sqldb, err := sql.Open("sqlite3", dbFile)
	if err != nil {
		panic(err)
	}

	sqlxDb := sqlx.NewDb(sqldb, "sqlite")
	return sqlxDb
}

var tmpDir = path.Clean(os.TempDir())
var dbPath = fmt.Sprintf("%s/flow_test", tmpDir)
var dbFile = fmt.Sprintf("%s/blobs.db", dbPath)

