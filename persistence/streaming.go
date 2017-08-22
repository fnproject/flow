package persistence

import (
	"github.com/AsynkronIT/protoactor-go/eventstream"
	"github.com/golang/protobuf/proto"
	"sync"
)

type StreamCallBack func(event *StreamEvent)

type StreamPredicate func(event *StreamEvent) bool

type StreamingProvider struct {
	state *streamingProviderState
}

type StreamingProviderState interface {
	ProviderState
	// StreamNewEvents sends any  events that match the predicate as they arrive to fn
	StreamNewEvents(predicate StreamPredicate, fn StreamCallBack) *eventstream.Subscription
	// SubscribeActorJournal streams all events that were persisted as they were saved by a given actor after fromIndex, and continues to stream any new events to the subscription
	SubscribeActorJournal(persistenceName string, fromIndex int, fn StreamCallBack) *eventstream.Subscription
	// QueryActorJournal searches the actor journal from a given index and sends all events that match predicate to fn
	QueryActorJournal(persistenceName string, fromIndex int, predicate StreamPredicate, fn StreamCallBack)
	// UnsubscribeStream closes a subscription
	UnsubscribeStream(sub *eventstream.Subscription)
}

type StreamEvent struct {
	ActorName  string
	EventIndex int
	Event      proto.Message
}

func (m *streamingProviderState) StreamNewEvents(predicate StreamPredicate, fn StreamCallBack) *eventstream.Subscription {
	return m.stream.Subscribe(func(e interface{}) {
		if event, ok := e.(*StreamEvent); ok {
			fn(event)
		}
	}).WithPredicate(func(e interface{}) bool {
		if evt, ok := e.(*StreamEvent); ok {
			return predicate(evt)
		}
		return false
	})
}

func (m *streamingProviderState) QueryActorJournal(persistenceName string, fromIndex int, predicate StreamPredicate, fn StreamCallBack) {
	m.GetEvents(persistenceName, fromIndex, func(idx int, e interface{}) {
		if event, ok := e.(proto.Message); ok {
			evt := &StreamEvent{ActorName: persistenceName, EventIndex: idx, Event: event}
			if predicate(evt) {
				fn(evt)
			}
		}
	})
}

func (m *streamingProviderState) SubscribeActorJournal(persistenceName string, fromIndex int, fn StreamCallBack) *eventstream.Subscription {

	type bufferedSub struct {
		lock           *sync.Mutex
		committed      bool
		bufferedEvents []*StreamEvent
		highestIndex   int
	}

	buffer := &bufferedSub{lock: &sync.Mutex{}, bufferedEvents: []*StreamEvent{}, highestIndex: -1}

	// Create a child subscription to buffer events while we read the journal
	childSub := m.stream.Subscribe(func(e interface{}) {
		if event, ok := e.(*StreamEvent); ok {
			buffer.lock.Lock()
			defer buffer.lock.Unlock()
			if buffer.committed {
				// replay - skip any messages we might have already replayed from storage
				if event.EventIndex > buffer.highestIndex {
					fn(event)
				}
			} else {
				buffer.bufferedEvents = append(buffer.bufferedEvents, event)

			}
		}
	}).WithPredicate(func(e interface{}) bool {
		if event, ok := e.(*StreamEvent); ok {
			return event.ActorName == persistenceName && event.EventIndex >= fromIndex
		}
		return false
	})

	// dump any pending events to the original fn
	m.GetEvents(persistenceName, fromIndex, func(idx int, e interface{}) {
		if event, ok := e.(proto.Message); ok {
			evt := &StreamEvent{ActorName: persistenceName, EventIndex: idx, Event: event}
			fn(evt)
			buffer.lock.Lock()
			buffer.highestIndex = fromIndex
			buffer.lock.Unlock()
		}
	})

	buffer.lock.Lock()
	defer buffer.lock.Unlock()
	for _, evt := range buffer.bufferedEvents {
		fn(evt)
	}
	buffer.committed = true
	return childSub
}

func (m *streamingProviderState) UnsubscribeStream(sub *eventstream.Subscription) {
	m.stream.Unsubscribe(sub)
}

// NewStreamingProvider wraps an existing provier to provide a stream on events
func NewStreamingProvider(target ProviderState) *StreamingProvider {
	return &StreamingProvider{newStreamingProviderState(target)}
}

// GetStreamingState returns the persistence.ProviderState associated with this provider
func (p *StreamingProvider) GetStreamingState() StreamingProviderState {
	return p.state
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
