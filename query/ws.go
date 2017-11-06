package query

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/AsynkronIT/protoactor-go/eventstream"
	"github.com/fnproject/flow/actor"
	"github.com/fnproject/flow/model"
	"github.com/fnproject/flow/persistence"
	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
	"github.com/gorilla/websocket"
)

type subscribeGraph struct {
	GraphID string `json:"graph_id"`
}

type unSubscribeGraph struct {
	GraphID string `json:"graph_id"`
}

func (sg *subscribeGraph) Action(l *wsWorker) error {
	if _, ok := l.subscriptions[sg.GraphID]; ok {
		// already subscribed nop
		return nil
	}

	log.WithField("conn", l.conn.LocalAddr().String()).WithField("graph_id", sg.GraphID).Info("Subscribed to graph")
	sub, err := l.manager.SubscribeGraphEvents(sg.GraphID, 0, func(e *persistence.StreamEvent) { l.SendGraphMessage(e, sg.GraphID) })
	if err != nil {
		return err
	}
	l.subscriptions[sg.GraphID] = sub
	return nil
}

func (sg *unSubscribeGraph) Action(l *wsWorker) error {
	if sub, ok := l.subscriptions[sg.GraphID]; ok {
		delete(l.subscriptions, sg.GraphID)
		l.manager.UnsubscribeStream(sub)
	}
	return nil
}

type jsonCommand struct {
	Command string `json:"command"`
}

var cmds = map[string](func() wsCommandHandler){
	"subscribe":   func() wsCommandHandler { return &subscribeGraph{} },
	"unsubscribe": func() wsCommandHandler { return &unSubscribeGraph{} },
}

func extractCommand(data []byte) (wsCommandHandler, error) {
	cmd := jsonCommand{}
	err := json.Unmarshal(data, &cmd)
	if err != nil {
		return nil, err
	}
	if cmdFactory, ok := cmds[cmd.Command]; ok {
		cmdObj := cmdFactory()
		err = json.Unmarshal([]byte(data), cmdObj)
		if err != nil {
			return nil, err
		}
		return cmdObj, nil
	}
	return nil, fmt.Errorf("unsupported command type %s", cmd.Command)

}

type wsCommandHandler interface {
	Action(listener *wsWorker) error
}

type wsWorker struct {
	conn          *websocket.Conn
	subscriptions map[string]*eventstream.Subscription
	marshaller    jsonpb.Marshaler
	manager       actor.GraphManager
}

type rawEventMsg struct {
	Type string          `json:"type"`
	Sub  string          `json:"sub"`
	Data json.RawMessage `json:"data"`
}

func (l *wsWorker) SendGraphMessage(event *persistence.StreamEvent, subscriptionID string) {
	body := event.Event
	protoType := proto.MessageName(body)

	bodyJSON, err := l.marshaller.MarshalToString(body)
	if err != nil {
		log.Warnf("Failed to convert to JSON: %s", err)
		return
	}
	msgJSON, err := json.Marshal(&rawEventMsg{Type: protoType, Data: json.RawMessage(bodyJSON), Sub: subscriptionID})

	if err != nil {
		log.Warnf("Failed to convert to JSON: %s", err)
		return
	}
	l.conn.WriteMessage(websocket.TextMessage, []byte(msgJSON))
}
func (l *wsWorker) Run() {

	defer l.Close()
	defer l.conn.Close()

	lifecycleEventPred := func(event *persistence.StreamEvent) bool {
		if !strings.HasPrefix(event.ActorName, "supervisor/") {
			return false
		}
		switch event.Event.(type) {
		case *model.GraphCreatedEvent:
			return true
		case *model.GraphCompletedEvent:
			return true
		}
		return false
	}

	subscriptionID := "_all"
	sub := l.manager.StreamNewEvents(lifecycleEventPred, func(e *persistence.StreamEvent) { l.SendGraphMessage(e, subscriptionID) })

	l.subscriptions[subscriptionID] = sub

	// main cmd loop
	for {
		msgType, msg, err := l.conn.ReadMessage()
		if msgType == websocket.TextMessage {
			cmd, err := extractCommand(msg)
			if err != nil {
				log.WithError(err).Errorf("Invalid command")
				break
			}

			err = cmd.Action(l)
			if err != nil {
				log.WithError(err).Errorf("Command Failed")
				break
			}

		}
		if err != nil || msgType == websocket.CloseMessage {
			break
		}
	}

}

func (l *wsWorker) Close() {
	for id, s := range l.subscriptions {
		log.Debugf("Unsubscribing %v from stream %s", l.conn.RemoteAddr(), id)
		l.manager.UnsubscribeStream(s)
	}
}

func newWorker(conn *websocket.Conn, manager actor.GraphManager) *wsWorker {
	return &wsWorker{conn: conn,
		subscriptions: make(map[string]*eventstream.Subscription),
		marshaller: jsonpb.Marshaler{EmitDefaults: true, OrigName: true},
		manager: manager}
}
