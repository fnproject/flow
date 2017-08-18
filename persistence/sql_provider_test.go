package persistence

import (
	"testing"
	"os"
	"net/url"
	"github.com/stretchr/testify/require"
	"github.com/fnproject/completer/model"
	_ "github.com/mattn/go-sqlite3"

	"fmt"
	"github.com/stretchr/testify/assert"
)

var tmpDir = os.TempDir()
var dbPath = fmt.Sprintf("%scompleter_test/", tmpDir)
var dbFile = fmt.Sprintf("%stest.db", dbPath)


func TestShouldFailToReadNonExistentSnapshot(t *testing.T) {
	provider := givenProvider(t)

	gotSnapshot, index, ok := provider.GetSnapshot("actorName")

	require.False(t, ok)
	assert.Equal(t, -1, index)
	assert.Nil(t,gotSnapshot)

}

func TestShouldReadAndWriteSnapshots(t *testing.T) {
	provider := givenProvider(t)
	snapshot := model.NewEmptyDatum()
	provider.PersistSnapshot("actorName", 1, snapshot)

	gotSnapshot, index, ok := provider.GetSnapshot("actorName")

	require.True(t, ok)
	assert.Equal(t, 1, index)

	assert.Equal(t, snapshot, gotSnapshot)
}

func TestSnapshotsOverrideOldOnes(t *testing.T) {
	provider := givenProvider(t)
	snapshot := model.NewBlobDatum(model.NewBlob("text/plain", []byte("hello")))
	provider.PersistSnapshot("actorName", 1, model.NewEmptyDatum())
	provider.PersistSnapshot("actorName", 2, snapshot)

	gotSnapshot, index, ok := provider.GetSnapshot("actorName")
	require.True(t, ok)
	assert.Equal(t, 2, index)
	assert.Equal(t, snapshot, gotSnapshot)

}

func givenProvider(t *testing.T) *SqlProvider {
	resetSqliteDb()
	provider, err := NewSqlProvider(dbUrl(), 0)
	require.NoError(t, err)
	return provider
}

func dbUrl() *url.URL {
	url, err := url.Parse("sqlite3://" + dbFile)
	if err != nil {
		panic(err)
	}
	return url
}
func resetSqliteDb() {
	os.RemoveAll(dbPath)
}
