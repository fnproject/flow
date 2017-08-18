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
	"path"
)

var tmpDir = path.Clean(os.TempDir())
var dbPath = fmt.Sprintf("%s/completer_test", tmpDir)
var dbFile = fmt.Sprintf("%s/test.db", dbPath)

func TestShouldFailToReadNonExistentSnapshot(t *testing.T) {
	provider := givenProvider(t)

	gotSnapshot, index, ok := provider.GetSnapshot("actorName")

	require.False(t, ok)
	assert.Equal(t, -1, index)
	assert.Nil(t, gotSnapshot)

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

func TestShouldReplayNoEventsForNewActor(t *testing.T) {
	provider := givenProvider(t)

	events := getEventsForActor(provider, "actorName", 0)

	assert.Empty(t, events)
}

func TestShouldWriteAnEventAndReplay(t *testing.T) {
	provider := givenProvider(t)
	event := model.NewBlobDatum(model.NewBlob("type", []byte("data")))

	provider.PersistEvent("actorName", 10, event)

	events := getEventsForActor(provider, "actorName", 0)

	assert.Equal(t, []*model.Datum{event}, events)
}

func TestShouldReplayEventsAfterIndex(t *testing.T) {
	provider := givenProvider(t)
	e1 := model.NewBlobDatum(model.NewBlob("type", []byte("1")))
	e2 := model.NewBlobDatum(model.NewBlob("type", []byte("2")))
	e3 := model.NewBlobDatum(model.NewBlob("type", []byte("3")))

	provider.PersistEvent("actorName", 1, e1)
	provider.PersistEvent("actorName", 2, e2)
	provider.PersistEvent("actorName", 3, e3)

	assert.Equal(t, []*model.Datum{e1, e2, e2}, getEventsForActor(provider, "actorName", 0))
	assert.Equal(t, []*model.Datum{e1, e2, e2}, getEventsForActor(provider, "actorName", 1))
	assert.Equal(t, []*model.Datum{e2, e2}, getEventsForActor(provider, "actorName", 2))
	assert.Equal(t, []*model.Datum{e1, e2, e2}, getEventsForActor(provider, "actorName", 0))
	assert.Equal(t, []*model.Datum{e1, e2, e2}, getEventsForActor(provider, "actorName", 1))
	assert.Equal(t, []*model.Datum{e2}, getEventsForActor(provider, "actorName", 3))
	assert.Equal(t, []*model.Datum{}, getEventsForActor(provider, "actorName", 4))

}

func getEventsForActor(provider *SqlProvider, actorName string, startIdx int) []*model.Datum {
	events := []*model.Datum{}
	provider.GetEvents(actorName, startIdx, func(e interface{}) {
		events = append(events, e.(*model.Datum))
	})
	return events
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
