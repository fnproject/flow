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

func TestCreateFirstSnapshots(t *testing.T) {
	provider := givenProvider(t)
	datum := model.NewEmptyDatum()
	provider.PersistSnapshot("actorName", 1, datum)

	snapshot, index, ok := provider.GetSnapshot("actorName")

	require.True(t, ok)
	assert.Equal(t, 1, index)

	assert.Equal(t, datum, snapshot)
}

func TestSnapshotsOverideOldOnes(t *testing.T) {
	provider := givenProvider(t)
	datum := model.NewBlobDatum(model.NewBlob("text/plain", []byte("hello"))
	provider.PersistSnapshot("actorName", 1, model.NewEmptyDatum())
	provider.PersistSnapshot("actorName", 2, datum)

	snapshot, index, ok := provider.GetSnapshot("actorName")
	require.True(t, ok)
	assert.Equal(t, 2, index)
	assert.Equal(t, datum, snapshot)

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
