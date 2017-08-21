package persistence
/**
   This is derived from vendor/github.com/AsynkronIT/protoactor-go/persistence/persistence_provider .go
   This has been modified to support propagating event indices to plugins
 */
import (
	proto "github.com/golang/protobuf/proto"
)

//Provider is the abstraction used for persistence
type Provider interface {
	GetState() ProviderState
}

type ProviderState interface {
	Restart()
	GetSnapshotInterval() int
	GetSnapshot(actorName string) (snapshot interface{}, eventIndex int, ok bool)
	GetEvents(actorName string, eventIndexStart int, callback func(messageIndex int ,e interface{}))
	PersistEvent(actorName string, eventIndex int, event proto.Message)
	PersistSnapshot(actorName string, eventIndex int, snapshot proto.Message)
}
