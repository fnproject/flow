package query

import (
	"encoding/json"
	"fmt"

	"github.com/fnproject/flow/model"
	"github.com/fnproject/flow/persistence"
	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
	"github.com/gorilla/websocket"
	"context"
)

type subscribeGraph struct {
	FlowID string `json:"flow_id"`
}

type unSubscribeGraph struct {
	FlowID string `json:"flow_id"`
}

func (sg *subscribeGraph) Action(l *wsWorker) error {
	if _, ok := l.subscriptions[sg.FlowID]; ok {
		// already subscribed nop
		return nil
	}

	log.WithField("conn", l.conn.LocalAddr().String()).WithField("flow_id", sg.FlowID).Info("Subscribed to graph")
/*
	req := &model.StreamGraphRequest{
		Query: &model.StreamGraphRequest_Graph{
			Graph: &model.StreamGraph{
				FlowId: sg.FlowID,
			},
		},
	}

	client, err := l.manager.StreamEvents(context.Background(), req)
	if err != nil {
		return err
	}
	go func() {
		for {
			msg, err := client.Recv()
			if err != nil {
				return
			}
			msg.GetVal()

			// TODO: send to graph
		}
	}()
*/
	//sub, err := l.manager.SubscribeGraphEvents(sg.FlowID, 0, func(e *persistence.StreamEvent) { l.SendGraphMessage(e, sg.FlowID) })

	l.subscriptions[sg.FlowID] = client
	return nil
}

func (sg *unSubscribeGraph) Action(l *wsWorker) error {
	if sub, ok := l.subscriptions[sg.FlowID]; ok {
		delete(l.subscriptions, sg.FlowID)
		sub.CloseSend()
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
	subscriptions map[string]model.FlowService_StreamEventsClient
	marshaller    jsonpb.Marshaler
	manager       model.FlowServiceClient
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

	//lifecycleEventPred := func(event *persistence.StreamEvent) bool {
	//	if !strings.HasPrefix(event.ActorName, "supervisor") {
	//		return false
	//	}
	//	switch event.Event.(type) {
	//	case *model.GraphCreatedEvent:
	//		return true
	//	case *model.GraphCompletedEvent:
	//		return true
	//	}
	//	return false
	//}

	// subscriptionID := "_all"

	return
	/*
	sub, err := l.manager.StreamEvents(context.Background(), &model.StreamRequest{Query: &model.StreamRequest_Lifecycle{}})

	// TODO send back lifecycle events.
	if err != nil {
		return
	}

	l.subscriptions[subscriptionID] = sub
	*/
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
		s.CloseSend()
	}
}

func newWorker(conn *websocket.Conn, manager model.FlowServiceClient) *wsWorker {
	return &wsWorker{conn: conn,
		subscriptions: make(map[string]model.FlowService_StreamEventsClient),
		marshaller: jsonpb.Marshaler{EmitDefaults: true, OrigName: true},
		manager: manager}
}
