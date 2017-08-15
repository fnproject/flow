package actor

import (
	"reflect"

	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/fnproject/completer/model"
	"github.com/sirupsen/logrus"
)

var (
	log = logrus.WithField("logger", "actor")
)

type graphSupervisor struct {
}

func (s *graphSupervisor) Receive(context actor.Context) {
	switch msg := context.Message().(type) {

	case *model.CreateGraphRequest:
		props := actor.FromInstance(&graphActor{})
		child, err := context.SpawnNamed(props, msg.GraphId)
		if err != nil {
			log.WithFields(logrus.Fields{"graph_id": msg.GraphId}).Warn("Failed to spawn graph actor")
			return
		}
		child.Request(msg, context.Sender())

	default:
		isGraphMsg, graphId := getGraphId(msg)
		if !isGraphMsg {
			return
		}
		found, child := findChild(context, graphId)
		if !found {
			log.WithFields(logrus.Fields{"graph_id": graphId}).Warn("No child actor found")
			return
		}
		child.Request(msg, context.Sender())
	}
}

func getGraphId(msg interface{}) (bool, string) {
	graphId := reflect.ValueOf(msg).Elem().FieldByName("GraphId")
	if graphId.IsValid() {
		return true, graphId.String()
	}
	return false, ""
}

func findChild(context actor.Context, graphId string) (bool, *actor.PID) {
	fullId := context.Self().Id + "/" + graphId
	for _, pid := range context.Children() {
		if pid.Id == fullId {
			return true, pid
		}
	}
	return false, nil
}
