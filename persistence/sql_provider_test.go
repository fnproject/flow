package persistence

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stretchr/testify/assert"
)

func TestShouldFailToReadNonExistentSnapshot(t *testing.T) {
	provider := givenProvider(t)

	gotSnapshot, index, ok := provider.GetSnapshot("actorName")

	require.False(t, ok)
	assert.Equal(t, -1, index)
	assert.Nil(t, gotSnapshot)

}

func testSnapshot(id string) *TestSnapshot {
	return &TestSnapshot{
		Index:     1,
		StringVal: id,
	}
}

func testEvent(id string) *TestEvent {
	return &TestEvent{
		Index:     1,
		StringVal: id,
	}
}

func TestShouldReadAndWriteSnapshots(t *testing.T) {
	provider := givenProvider(t)
	snapshot := testSnapshot("test")
	provider.PersistSnapshot("actorName", 1, snapshot)

	gotSnapshot, index, ok := provider.GetSnapshot("actorName")

	require.True(t, ok)
	assert.Equal(t, 1, index)
	assert.Equal(t, snapshot, gotSnapshot)
}

func TestSnapshotsOverrideOldOnes(t *testing.T) {
	provider := givenProvider(t)
	snapshot := testSnapshot("new")
	provider.PersistSnapshot("actorName", 1, testSnapshot("old"))
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
	event := testEvent("data")

	provider.PersistEvent("actorName", 10, event)

	events := getEventsForActor(provider, "actorName", 0)

	assert.Equal(t, []*TestEvent{event}, events)
}

func TestShouldReplayEventsAfterIndex(t *testing.T) {
	provider := givenProvider(t)
	e1 := testEvent("1")
	e2 := testEvent("2")
	e3 := testEvent("3")

	provider.PersistEvent("actorName", 1, e1)
	provider.PersistEvent("actorName", 2, e2)
	provider.PersistEvent("actorName", 3, e3)

	assert.Equal(t, []*TestEvent{e1, e2, e3}, getEventsForActor(provider, "actorName", 0))
	assert.Equal(t, []*TestEvent{e1, e2, e3}, getEventsForActor(provider, "actorName", 1))
	assert.Equal(t, []*TestEvent{e2, e3}, getEventsForActor(provider, "actorName", 2))
	assert.Equal(t, []*TestEvent{e1, e2, e3}, getEventsForActor(provider, "actorName", 0))
	assert.Equal(t, []*TestEvent{e1, e2, e3}, getEventsForActor(provider, "actorName", 1))
	assert.Equal(t, []*TestEvent{e3}, getEventsForActor(provider, "actorName", 3))
	assert.Equal(t, []*TestEvent{}, getEventsForActor(provider, "actorName", 4))

}

func getEventsForActor(provider ProviderState, actorName string, startIdx int) []*TestEvent {
	events := []*TestEvent{}
	provider.GetEvents(actorName, startIdx, func(_ int, e interface{}) {
		events = append(events, e.(*TestEvent))
	})
	return events
}

func givenProvider(t *testing.T) ProviderState {
	resetTestDb()
	db, err := CreateDBConnection(testDBURL())
	require.NoError(t, err)
	provider, err := NewSQLProvider(db, 0)
	require.NoError(t, err)
	return provider
}
