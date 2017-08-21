package query

import (
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/AsynkronIT/protoactor-go/eventstream"
	"github.com/golang/protobuf/jsonpb"
	"github.com/fnproject/completer/actor"
	"github.com/fnproject/completer/model"
	"github.com/fnproject/completer/persistence"
	"github.com/golang/protobuf/proto"
	"encoding/json"
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

	log.WithField("conn",l.conn.LocalAddr().String()).WithField("graph_id",sg.GraphID).Info("Subscribed to graph")
	sub := l.manager.SubscribeGraph(sg.GraphID, 0, l.SendGraphMessage)
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
	"subscribe": func() (wsCommandHandler) { return &subscribeGraph{} },
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
	} else {
		return nil, fmt.Errorf("Unsupported command type %s", cmd.Command)
	}

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

type graphMesssage struct {
	Type string `json:"type"`
	Data json.RawMessage `json:"data"`
}

func (l *wsWorker) SendGraphMessage(event *persistence.StreamEvent) {
	body := event.Event
	protoType := proto.MessageName(body)

	bodyjson, err := l.marshaller.MarshalToString(body)
	if err != nil {
		log.Warnf("Failed to convert to JSON: %s", err)
		return
	}
	msgJson, err := json.Marshal(&graphMesssage{Type: protoType, Data: json.RawMessage(bodyjson)})

	if err != nil {
		log.Warnf("Failed to convert to JSON: %s", err)
		return
	}
	l.conn.WriteMessage(websocket.TextMessage, []byte(msgJson))
}
func (l *wsWorker) Run() {

	defer l.Close()
	defer l.conn.Close()

	sub := l.manager.SubscribeGraph("*", 0, l.SendGraphMessage).WithPredicate(
		func(event interface{}) bool {
			if evt, ok := event.(*persistence.StreamEvent); ok {
				switch evt.Event.(type) {
				case *model.GraphCreatedEvent:
					return true
				case *model.GraphCompletedEvent:
					return true
				}
			}
			return false
		})

	l.subscriptions["*"] = sub

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
