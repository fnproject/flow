package actor

import (
	"fmt"
	"reflect"

	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/fnproject/completer/model"
	"github.com/sirupsen/logrus"
)

type graphSupervisor struct {
	log *logrus.Entry
}

func newGraphSupervisor() *graphSupervisor {
	return &graphSupervisor{
		log: logrus.WithField("logger", "graphSupervisor"),
	}
}

func (s *graphSupervisor) Receive(context actor.Context) {
	switch msg := context.Message().(type) {
	case *model.CreateGraphRequest:
		s.log.WithFields(logrus.Fields{"graph_id": msg.GraphId}).Info("Creating graph")
		props := actor.FromInstance(&graphActor{})
		child, err := context.SpawnNamed(props, msg.GraphId)
		if err != nil {
			child.Request(msg, context.Sender())
		}
	default:
		graphId := reflect.ValueOf(msg).Elem().FieldByName("GraphId").String()
		found, child := findChild(context, graphId)
		if !found {
			s.log.WithFields(logrus.Fields{"graph_id": msg.GraphId}).Warn("No child actor found!")
			return
		}
		child.Request(msg, context.Sender())
	}
}

func findChild(context actor.Context, graphId string) (bool, *actor.PID) {
	fullId := context.Self().Id + "/" + graphId
	for _, pid := range context.Children() {
		fmt.Printf("child %s\n", pid.Id)
		if pid.Id == fullId {
			return true, pid
		}
	}
	return false, nil
}

type graphActor struct {
}

func (g *graphActor) Receive(context actor.Context) {
	switch msg := context.Message().(type) {
	case *model.CreateGraphRequest:
		fmt.Printf("Creating graph %s", msg.GraphId)
	case *model.CommitGraphRequest:
		fmt.Printf("Committing graph %s", msg.GraphId)
		context.Respond(&model.CommitGraphProcessed{GraphId: msg.GraphId})
	}
}
