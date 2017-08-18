package actor

import (
	"reflect"

	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/persistence"
	"github.com/AsynkronIT/protoactor-go/plugin"
	"github.com/AsynkronIT/protoactor-go/stream"
	"github.com/fnproject/completer/model"
	"github.com/sirupsen/logrus"
)

var (
	log = logrus.WithField("logger", "actor")
)

type graphSupervisor struct {
	executor    *actor.PID
	eventStream *stream.UntypedStream
}

// NewSupervisor creates new graphSupervisor actor
func NewSupervisor(executor *actor.PID, eventStream *stream.UntypedStream) actor.Actor {
	return &graphSupervisor{executor: executor, eventStream: eventStream}
}

func (s *graphSupervisor) Receive(context actor.Context) {
	switch msg := context.Message().(type) {

	case *model.CreateGraphRequest:
		provider := newInMemoryProvider(1000)
		props := actor.
			FromInstance(NewGraphActor(msg.GraphId, msg.FunctionId, s.executor)).
			WithMiddleware(
				plugin.Use(&PIDAwarePlugin{}),
				persistence.Using(provider),
				plugin.Use(&EventStreamPlugin{stream: s.eventStream}),
			)

		child, err := context.SpawnNamed(props, msg.GraphId)
		if err != nil {
			log.WithFields(logrus.Fields{"graph_id": msg.GraphId}).Warn("Failed to spawn graph actor")
			context.Respond(NewGraphCreationError(msg.GraphId))
			return
		}
		log.Infof("Created graph actor %s", child.Id)
		child.Request(msg, context.Sender())

	default:
		if isGraphMessage(msg) {
			graphID := getGraphID(msg)
			child, found := findChild(context, graphID)
			if !found {
				log.WithFields(logrus.Fields{"graph_id": graphID}).Warn("No child actor found")
				context.Respond(NewGraphNotFoundError(graphID))
				return
			}
			child.Request(msg, context.Sender())
		}
	}
}

func isGraphMessage(msg interface{}) bool {
	return reflect.ValueOf(msg).Elem().FieldByName("GraphId").IsValid()
}

func getGraphID(msg interface{}) string {
	return reflect.ValueOf(msg).Elem().FieldByName("GraphId").String()
}

func findChild(context actor.Context, graphID string) (*actor.PID, bool) {
	fullID := context.Self().Id + "/" + graphID
	for _, pid := range context.Children() {
		if pid.Id == fullID {
			return pid, true
		}
	}
	return nil, false
}
