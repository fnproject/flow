package actor

import (
	"reflect"
	"strings"

	"github.com/AsynkronIT/protoactor-go/actor"
	protoPersistence "github.com/AsynkronIT/protoactor-go/persistence"
	"github.com/AsynkronIT/protoactor-go/plugin"
	"github.com/fnproject/completer/model"
	"github.com/fnproject/completer/persistence"
	"github.com/sirupsen/logrus"
)

type graphSupervisor struct {
	executor            *actor.PID
	persistenceProvider persistence.Provider
	log                 *logrus.Entry
	persistence.Mixin
	activeGraphs map[string]bool
}

// NewSupervisor creates new graphSupervisor actor
func NewSupervisor(executor *actor.PID, persistenceProvider persistence.Provider) actor.Actor {
	return &graphSupervisor{
		executor:            executor,
		persistenceProvider: persistenceProvider,
		log:                 logrus.New().WithField("logger", "graph_supervisor"),
		activeGraphs:        make(map[string]bool),
	}
}

func (s *graphSupervisor) Receive(context actor.Context) {
	if s.Recovering() {
		s.receiveEvent(context)
	} else {
		s.receiveCommand(context)
	}
}

func (s *graphSupervisor) handleActiveGraph(graphID string) {
	s.log.WithField("graph_id", graphID).Debug("Adding active graph")
	s.activeGraphs[graphID] = true
}

func (s *graphSupervisor) handleInactiveGraph(graphID string) {
	s.log.WithField("graph_id", graphID).Debug("Removing inactive graph")
	delete(s.activeGraphs, graphID)
}

func (s *graphSupervisor) receiveCommand(context actor.Context) {
	switch msg := context.Message().(type) {

	case *model.CreateGraphRequest:
		child, err := s.spawnGraphActor(context, msg.GraphId)
		if err != nil {
			s.log.WithFields(logrus.Fields{"graph_id": msg.GraphId}).Warn("Failed to spawn graph actor")
			context.Respond(model.NewGraphCreationError(msg.GraphId))
			return
		}
		s.PersistReceive(&model.GraphCreatedEvent{GraphId: msg.GraphId})
		s.handleActiveGraph(msg.GraphId)
		child.Request(msg, context.Sender())

	case *model.DeactivateGraphRequest:
		if s.activeGraphs[msg.GraphId] {
			s.PersistReceive(&model.GraphCompletedEvent{GraphId: msg.GraphId})
			s.handleInactiveGraph(msg.GraphId)
		}
		if child, ok := s.findChild(context, msg.GraphId); ok {
			child.Tell(&actor.PoisonPill{})
		}

	case model.GraphMessage:
		child, err := s.getGraphActor(context, msg.GetGraphId())
		if err != nil {
			s.log.WithFields(logrus.Fields{"graph_id": msg.GetGraphId()}).Warn("No child actor found")
			context.Respond(model.NewGraphNotFoundError(msg.GetGraphId()))
			return
		}
		child.Request(msg, context.Sender())

	case *actor.Terminated:
		if graphID, ok := getGraphID(msg.GetWho()); ok {
			s.log.WithFields(logrus.Fields{"graph_id": graphID}).Info("Graph actor terminated")
			if s.activeGraphs[graphID] {
				s.log.WithFields(logrus.Fields{"graph_id": graphID}).Warn("Graph actor crashed")
				// TODO re-spawn failed graph actor and tell it to set its status to error
			}
		}

	case *protoPersistence.ReplayComplete:
		s.log.Infof("Respawning %d active graphs", len(s.activeGraphs))
		for graphID := range s.activeGraphs {
			s.spawnGraphActor(context, graphID)
		}
	}
}

func getGraphID(child *actor.PID) (string, bool) {
	split := strings.Split(child.Id, supervisorName+"/")
	if len(split) == 2 && len(split[1]) > 0 {
		return split[1], true
	}
	return "", false
}

func (s *graphSupervisor) receiveEvent(context actor.Context) {
	switch msg := context.Message().(type) {

	case *model.GraphCreatedEvent:
		s.handleActiveGraph(msg.GraphId)

	case *model.GraphCompletedEvent:
		s.handleInactiveGraph(msg.GraphId)

	default:
		s.log.Infof("Ignoring replayed message of unknown type %v", reflect.TypeOf(msg))
	}
}

// this method will spawn a graph actor if it doesn't already exist
func (s *graphSupervisor) getGraphActor(context actor.Context, graphID string) (*actor.PID, error) {
	if child, ok := s.findChild(context, graphID); ok {
		return child, nil
	}
	return s.spawnGraphActor(context, graphID)
}

func (s *graphSupervisor) findChild(context actor.Context, graphID string) (*actor.PID, bool) {
	fullID := context.Self().Id + "/" + graphID
	for _, pid := range context.Children() {
		if pid.Id == fullID {
			return pid, true
		}
	}
	return nil, false
}

func (s *graphSupervisor) spawnGraphActor(context actor.Context, graphID string) (*actor.PID, error) {
	props := actor.
		FromInstance(NewGraphActor(s.executor)).
		WithMiddleware(
			plugin.Use(&PIDAwarePlugin{}),
			persistence.Using(s.persistenceProvider),
		)
	pid, err := context.SpawnNamed(props, graphID)
	context.Watch(pid)
	s.log.WithFields(logrus.Fields{"graph_id": graphID}).Infof("Created graph actor %s", pid.Id)
	return pid, err
}
