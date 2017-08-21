package actor

import (
	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/plugin"
	"github.com/fnproject/completer/model"
	"github.com/sirupsen/logrus"
	"github.com/fnproject/completer/persistence"
)

type graphSupervisor struct {
	executor            *actor.PID
	persistenceProvider persistence.Provider
	log                 *logrus.Entry
}

// NewSupervisor creates new graphSupervisor actor
func NewSupervisor(executor *actor.PID, persistenceProvider persistence.Provider) actor.Actor {
	return &graphSupervisor{executor: executor, persistenceProvider: persistenceProvider, log: logrus.New().WithField("logger", "graph_supervisor")}
}

func (s *graphSupervisor) Receive(context actor.Context) {

	if gm, ok := context.Message().(**model.CreateGraphRequest); ok {
		s.log.Infof("Created graph actor %s", (*gm).GetGraphId())
	}

	if gm, ok := context.Message().(*model.GraphMessage); ok {
		s.log.Infof("Yup actor %s", (*gm).GetGraphId())

	}
	switch msg := context.Message().(type) {

	case *model.CreateGraphRequest:
		props := actor.
		FromInstance(NewGraphActor(msg.GraphId, msg.FunctionId, s.executor)).
			WithMiddleware(
			plugin.Use(&PIDAwarePlugin{}),
			persistence.Using(s.persistenceProvider),
		)

		child, err := context.SpawnNamed(props, msg.GraphId)
		if err != nil {
			s.log.WithFields(logrus.Fields{"graph_id": msg.GraphId}).Warn("Failed to spawn graph actor")
			context.Respond(NewGraphCreationError(msg.GraphId))
			return
		}
		s.log.Infof("Created graph actor %s", child.Id)
		child.Request(msg, context.Sender())

	case model.GraphMessage:

		graphID := msg.GetGraphId()
		child, found := findChild(context, graphID)
		if !found {
			s.log.WithFields(logrus.Fields{"graph_id": graphID}).Warn("No child actor found")
			context.Respond(NewGraphNotFoundError(graphID))
			return
		}
		child.Request(msg, context.Sender())

	}
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
