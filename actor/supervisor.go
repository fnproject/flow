package actor

import (
	"reflect"

	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/persistence"
	"github.com/AsynkronIT/protoactor-go/plugin"
	"github.com/fnproject/completer/model"
	"github.com/sirupsen/logrus"
)

var (
	log = logrus.WithField("logger", "actor")
)

type graphSupervisor struct {
	executor *actor.PID
}

// NewSupervisor creates new graphSupervisor actor
func NewSupervisor(executor *actor.PID) actor.Actor {
	return &graphSupervisor{executor: executor}
}

func (s *graphSupervisor) Receive(context actor.Context) {
	switch msg := context.Message().(type) {

	case *model.CreateGraphRequest:
		provider := newInMemoryProvider(1000)
		props := actor.FromInstance(NewGraphActor(msg.GraphId, msg.FunctionId, s.executor)).WithMiddleware(plugin.Use(&PIDAwarePlugin{}), persistence.Using(provider))
		log.Info("Creating graph actor")
		child, err := context.SpawnNamed(props, msg.GraphId)
		if err != nil {
			log.WithFields(logrus.Fields{"graph_id": msg.GraphId}).Warn("Failed to spawn graph actor")
			return
		}
		child.Request(msg, context.Sender())

	default:
		if isGraphMessage(msg) {
			graphId := getGraphId(msg)
			child, found := findChild(context, graphId)
			if !found {
				log.WithFields(logrus.Fields{"graph_id": graphId}).Warn("No child actor found")
				context.Respond(NewGraphNotFoundError(graphId))
				return
			}
			child.Request(msg, context.Sender())
		}
	}
}

func isGraphMessage(msg interface{}) bool {
	return reflect.ValueOf(msg).Elem().FieldByName("GraphId").IsValid()
}

func getGraphId(msg interface{}) string {
	return reflect.ValueOf(msg).Elem().FieldByName("GraphId").String()
}

func findChild(context actor.Context, graphId string) (*actor.PID, bool) {
	fullId := context.Self().Id + "/" + graphId
	for _, pid := range context.Children() {
		if pid.Id == fullId {
			return pid, true
		}
	}
	return nil, false
}
