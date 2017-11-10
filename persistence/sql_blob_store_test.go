package persistence

import (
	"github.com/fnproject/flow/model"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"bytes"
)

func TestShouldInsertBlobAndGenerateId(t *testing.T) {
	store := givenEmptyBlobStore()

	data := []byte("Some data")
	blob, err := store.Create("graph","test/type", bytes.NewReader(data))

	assert.NoError(t, err)
	require.NotNil(t, blob)
	assert.NotNil(t, blob.BlobId)
	assert.Equal(t, "test/type", blob.ContentType)
	assert.Equal(t, uint64(len(data)), blob.Length)

}

func TestShouldRetrieveStoredBlob(t *testing.T) {
	store := givenEmptyBlobStore()

	data := []byte("Some data")
	blob, err := store.Create("graph","test/type", bytes.NewReader(data))
	require.NoError(t, err)

	newData, err := store.Read("graph",blob)
	assert.NoError(t, err)
	assert.Equal(t, data, newData)

}

func TestShouldFailWithUnknownBlob(t *testing.T) {
	store := givenEmptyBlobStore()

	newData, err := store.Read("graph",&model.BlobDatum{BlobId: "foo"})
	assert.Nil(t, newData)
	assert.Error(t, err)

}
func TestShouldReadAndWriteEmptyBlob(t *testing.T) {
	store := givenEmptyBlobStore()

	blob, err := store.Create("graph","test/type", bytes.NewReader([]byte{}))
	require.NoError(t, err)
	assert.Equal(t, uint64(0), blob.Length)

	data, err := store.Read("graph",blob)
	assert.NoError(t, err)
	assert.Empty(t, data)
}

func givenEmptyBlobStore() BlobStore {

	db := setupDb()
	store, err := NewSQLBlobStore(db)
	if err != nil {
		panic(err)
	}
	return store
}

func setupDb() *sqlx.DB {
	resetTestDb()

	db, err := CreateDBConnection(testDbURL())
	if err != nil {
		panic(err)
	}
	return db
}
