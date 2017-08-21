package persistence

import (
	"github.com/AsynkronIT/protoactor-go/eventstream"
	"github.com/golang/protobuf/proto"
)

type StreamingProvider struct {
	state *streamingProviderState
}

type StreamEvent struct {
	ActorName  string
	EventIndex int
	Event      proto.Message
}

// NewStreamingProvider wraps an existing provier to provide a stream on events
func NewStreamingProvider(target ProviderState) *StreamingProvider {
	return &StreamingProvider{newStreamingProviderState(target)}
}

// GetState returns the persistence.ProviderState associated with this provider
func (p *StreamingProvider) GetState() ProviderState {
	return p.state
}

// GetState returns the persistence.ProviderState associated with this provider
func (p *StreamingProvider) GetEventStream() *eventstream.EventStream {
	return p.state.stream
}

// decorates persistence.Provider by publishing persisted events to the associated EventStream
type streamingProviderState struct {
	ProviderState
	stream *eventstream.EventStream
}

func newStreamingProviderState(target ProviderState) *streamingProviderState {
	return &streamingProviderState{ProviderState: target, stream: &eventstream.EventStream{}}
}

func (s *streamingProviderState) PersistEvent(actorName string, eventIndex int, event proto.Message) {
	s.ProviderState.PersistEvent(actorName, eventIndex, event)
	s.stream.Publish(&StreamEvent{ActorName: actorName, EventIndex: eventIndex, Event: event})
}
