package persistence

import (
	"sync"

	"github.com/golang/protobuf/proto"
)

type entry struct {
	eventIndex int // the event index right after snapshot
	snapshot   proto.Message
	events     []proto.Message
}

type SqlProvider struct {
	snapshotInterval int
	mu               sync.RWMutex
	store            map[string]*entry // actorName -> a persistence entry
}

func NewSqlProvider(snapshotInterval int,db *sqlx.DB) *SqlProvider {
	return &SqlProvider{
		snapshotInterval: snapshotInterval,
		store:            make(map[string]*entry),
	}
}



func (provider *SqlProvider) Restart() {}

func (provider *SqlProvider) GetSnapshotInterval() int {
	return provider.snapshotInterval
}

func (provider *SqlProvider) GetSnapshot(actorName string) (snapshot interface{}, eventIndex int, ok bool) {
	return nil,0,false
}

func (provider *SqlProvider) PersistSnapshot(actorName string, eventIndex int, snapshot proto.Message) {

}

func (provider *SqlProvider) GetEvents(actorName string, eventIndexStart int, callback func(e interface{})) {

}

func (provider *SqlProvider) PersistEvent(actorName string, eventIndex int, event proto.Message) {

}
