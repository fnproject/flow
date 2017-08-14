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
		log.WithFields(logrus.Fields{"graph_id": msg.GraphId}).Info("Creating graph")
		props := actor.FromInstance(&graphActor{})
		child, err := context.SpawnNamed(props, msg.GraphId)
		if err != nil {
			child.Request(msg, context.Sender())
		}

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

type graphActor struct {
}

func (g *graphActor) Receive(context actor.Context) {
	switch msg := context.Message().(type) {

	case *model.CreateGraphRequest:
		log.WithFields(logrus.Fields{"graph_id": msg.GraphId}).Info("Creating graph")

	case *model.AddChainedStageRequest:
		log.WithFields(logrus.Fields{"graph_id": msg.GraphId}).Info("Adding chained stage")

	case *model.AddCompletedValueStageRequest:
		log.WithFields(logrus.Fields{"graph_id": msg.GraphId}).Info("Adding completed value stage")

	case *model.AddDelayStageRequest:
		log.WithFields(logrus.Fields{"graph_id": msg.GraphId}).Info("Adding delay stage")

	case *model.AddExternalCompletionStageRequest:
		log.WithFields(logrus.Fields{"graph_id": msg.GraphId}).Info("Adding external completion stage")

	case *model.AddInvokeFunctionStageRequest:
		log.WithFields(logrus.Fields{"graph_id": msg.GraphId}).Info("Adding invoke stage")

	case *model.CompleteStageExternallyRequest:
		log.WithFields(logrus.Fields{"graph_id": msg.GraphId}).Info("Completing stage externally")

	case *model.CommitGraphRequest:
		log.WithFields(logrus.Fields{"graph_id": msg.GraphId}).Info("Committing graph")
		context.Respond(&model.CommitGraphProcessed{GraphId: msg.GraphId})

	case *model.GetStageResultRequest:
		log.WithFields(logrus.Fields{"graph_id": msg.GraphId}).Info("Retrieving stage result")

	case *model.CompleteDelayStageRequest:
		log.WithFields(logrus.Fields{"graph_id": msg.GraphId}).Info("Completing delayed stage")

	case *model.FaasInvocationResponse:
		log.WithFields(logrus.Fields{"graph_id": msg.GraphId}).Info("Received fn invocation response")
	}

}
