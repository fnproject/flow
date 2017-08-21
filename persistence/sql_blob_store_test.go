package persistence

import (
	"testing"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/fnproject/completer/model"
)

func TestShouldInsertBlobAndGenerateId(t *testing.T) {
	store := givenEmptyBlobStore()

	data := []byte("Some data")
	blob, err := store.CreateBlob("test/type", data)

	assert.NoError(t, err)
	require.NotNil(t, blob)
	assert.NotNil(t, blob.BlobId)
	assert.Equal(t, "test/type", blob.ContentType)
	assert.Equal(t, uint64(len(data)), blob.Length)

}

func TestShouldRetrieveStoredBlob(t *testing.T) {
	store := givenEmptyBlobStore()

	data := []byte("Some data")
	blob, err := store.CreateBlob("test/type", data)
	require.NoError(t, err)

	newData, err := store.ReadBlobData(blob)
	assert.NoError(t, err)
	assert.Equal(t, data, newData)

}

func TestShouldFailWithUnknownBlob(t *testing.T) {
	store := givenEmptyBlobStore()

	newData, err := store.ReadBlobData(&model.BlobDatum{BlobId: "foo"})
	assert.Nil(t, newData)
	assert.Error(t, err)

}
func TestShouldReadAndWriteEmptyBlob(t *testing.T) {
	store := givenEmptyBlobStore()

	blob, err := store.CreateBlob("test/type", []byte{})
	require.NoError(t, err)
	assert.Equal(t, uint64(0), blob.Length)

	data, err := store.ReadBlobData(blob)
	assert.NoError(t, err)
	assert.Empty(t, data)
}

func givenEmptyBlobStore() BlobStore {

	db := setupDb()
	store, err := NewSqlBlobStore(db)
	if err != nil {
		panic(err)
	}
	return store
}
func setupDb() *sqlx.DB {
	resetTestDb()

	db, err := CreateDBConnecection(testDbUrl())
	if err != nil {
		panic(err)
	}
	return db
}
