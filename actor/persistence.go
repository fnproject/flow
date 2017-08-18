package actor

import (
	"github.com/AsynkronIT/protoactor-go/eventstream"
	"github.com/AsynkronIT/protoactor-go/persistence"
	proto "github.com/golang/protobuf/proto"
)

type streamingInMemoryProvider struct {
	state *streamingProviderState
}

func newStreamingInMemoryProvider(snapshotInterval int) *streamingInMemoryProvider {
	return &streamingInMemoryProvider{newStreamingProviderState(snapshotInterval)}
}

// GetState returns the persistence.ProviderState associated with this provider
func (p *streamingInMemoryProvider) GetState() persistence.ProviderState {
	return p.state
}

// GetState returns the persistence.ProviderState associated with this provider
func (p *streamingInMemoryProvider) GetEventStream() *eventstream.EventStream {
	return p.state.stream
}

// decorates persistence.Provider by publishing persisted events to the associated EventStream
type streamingProviderState struct {
	persistence.ProviderState
	stream *eventstream.EventStream
}

func newStreamingProviderState(snapshotInterval int) *streamingProviderState {
	return &streamingProviderState{persistence.NewInMemoryProvider(snapshotInterval), &eventstream.EventStream{}}
}

func (s *streamingProviderState) PersistEvent(actorName string, eventIndex int, event proto.Message) {
	s.ProviderState.PersistEvent(actorName, eventIndex, event)
	s.stream.Publish(event)
}
