package persistence

/**
  This is derived from vendor/github.com/AsynkronIT/protoactor-go/persistence/in_memory_provider.go
  This has been modified to support propagating event indices to plugins
*/
import (
	"github.com/golang/protobuf/proto"
	"sync"
)

type entry struct {
	eventIndex int // the event index right after snapshot
	snapshot   proto.Message
	events     []proto.Message
}

// InMemoryProvider is a proto.actor persistence provider
type InMemoryProvider struct {
	snapshotInterval int
	mu               sync.RWMutex
	store            map[string]*entry // actorName -> a persistence entry
}

// NewInMemoryProvider creates a new in mem provider
func NewInMemoryProvider(snapshotInterval int) ProviderState {
	return &InMemoryProvider{
		snapshotInterval: snapshotInterval,
		store:            make(map[string]*entry),
	}
}

func (provider *InMemoryProvider) loadOrInit(actorName string) (e *entry, loaded bool) {
	provider.mu.RLock()
	e, ok := provider.store[actorName]
	provider.mu.RUnlock()

	if !ok {
		provider.mu.Lock()
		e = &entry{}
		provider.store[actorName] = e
		provider.mu.Unlock()
	}

	return e, ok
}

// Restart implements ProviderStage.Restart
func (provider *InMemoryProvider) Restart() {}

// GetSnapshotInterval implements ProviderState.GetSnapshotInterval
func (provider *InMemoryProvider) GetSnapshotInterval() int {
	return provider.snapshotInterval
}

// GetSnapshot implements ProviderState.GetSnapshot
func (provider *InMemoryProvider) GetSnapshot(actorName string) (snapshot interface{}, eventIndex int, ok bool) {
	entry, loaded := provider.loadOrInit(actorName)
	if !loaded || entry.snapshot == nil {
		return nil, 0, false
	}
	return entry.snapshot, entry.eventIndex, true
}

// PersistSnapshot implements ProviderState.PersistSnapshot
func (provider *InMemoryProvider) PersistSnapshot(actorName string, eventIndex int, snapshot proto.Message) {
	entry, _ := provider.loadOrInit(actorName)
	entry.eventIndex = eventIndex
	entry.snapshot = snapshot
}

// GetEvents implements ProviderState.GetEvents
func (provider *InMemoryProvider) GetEvents(actorName string, eventIndexStart int, callback func(index int, e interface{})) {
	entry, _ := provider.loadOrInit(actorName)
	for idx, e := range entry.events[eventIndexStart:] {
		callback(idx, e)
	}
}

// PersistEvent implements ProviderState.PersistEvent
func (provider *InMemoryProvider) PersistEvent(actorName string, eventIndex int, event proto.Message) {
	entry, _ := provider.loadOrInit(actorName)
	entry.events = append(entry.events, event)
}
