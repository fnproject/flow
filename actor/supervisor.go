package actor

import (
	"reflect"
	"strings"

	"github.com/AsynkronIT/protoactor-go/actor"
	protoPersistence "github.com/AsynkronIT/protoactor-go/persistence"
	"github.com/AsynkronIT/protoactor-go/plugin"
	"github.com/fnproject/flow/model"
	"github.com/fnproject/flow/persistence"
	"github.com/sirupsen/logrus"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	activeGraphsMetric = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "flow_concurrent_active_graphs",
		Help: "Currently active graphs.",
	})
)

func init() {
	prometheus.MustRegister(activeGraphsMetric)
	activeGraphsMetric.Set(0.0)
}

type graphSupervisor struct {
	executor            *actor.PID
	persistenceProvider persistence.Provider
	log                 *logrus.Entry
	persistence.Mixin
	// TODO turn this into a pid cache to avoid iterating on all children?
	activeGraphs map[string]bool
}

// NewSupervisor creates new graphSupervisor actor
func NewSupervisor(name string, executor *actor.PID, persistenceProvider persistence.Provider) actor.Actor {
	return &graphSupervisor{
		executor:            executor,
		persistenceProvider: persistenceProvider,
		log:                 logrus.New().WithField("logger", name),
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

func (s *graphSupervisor) handleActiveGraph(flowID string) {
	s.log.WithField("flow_id", flowID).Debug("Adding active graph")
	s.activeGraphs[flowID] = true
	activeGraphsMetric.Inc()
}

func (s *graphSupervisor) handleInactiveGraph(flowID string) {
	s.log.WithField("flow_id", flowID).Debug("Removing inactive graph")
	delete(s.activeGraphs, flowID)
	activeGraphsMetric.Dec()
}

func (s *graphSupervisor) receiveCommand(context actor.Context) {
	switch msg := context.Message().(type) {

	case *model.CreateGraphRequest:
		child, err := s.spawnGraphActor(context, msg.FlowId)
		if err != nil {
			s.log.WithFields(logrus.Fields{"flow_id": msg.FlowId}).Warn("Failed to spawn graph actor")
			context.Respond(model.NewGraphCreationError(msg.FlowId))
			return
		}
		s.PersistReceive(&model.GraphCreatedEvent{FlowId: msg.FlowId})
		s.handleActiveGraph(msg.FlowId)
		child.Request(msg, context.Sender())

	case *model.DeactivateGraphRequest:
		if s.activeGraphs[msg.FlowId] {
			s.PersistReceive(&model.GraphCompletedEvent{FlowId: msg.FlowId})
			s.handleInactiveGraph(msg.FlowId)
		}
		if child, ok := s.findChild(context, msg.FlowId); ok {
			child.Tell(&actor.PoisonPill{})
		}

	case model.GraphMessage:
		child, err := s.getGraphActor(context, msg.GetFlowId())
		if err != nil {
			s.log.WithFields(logrus.Fields{"flow_id": msg.GetFlowId()}).Warn("No child actor found")
			context.Respond(model.NewGraphNotFoundError(msg.GetFlowId()))
			return
		}
		child.Request(msg, context.Sender())

	case *actor.Terminated:
		if flowID, ok := getFlowID(context.Self().Id, msg.GetWho()); ok {
			s.log.WithFields(logrus.Fields{"flow_id": flowID}).Info("Graph actor terminated")
			if s.activeGraphs[flowID] {
				s.log.WithFields(logrus.Fields{"flow_id": flowID}).Warn("Graph actor crashed")
				// TODO re-spawn failed graph actor and tell it to set its status to error
			}
		}

	case *protoPersistence.ReplayComplete:
		s.log.Infof("Respawning %d active graphs", len(s.activeGraphs))
		for flowID := range s.activeGraphs {
			s.spawnGraphActor(context, flowID)
		}
	}
}

func getFlowID(supervisorName string, child *actor.PID) (string, bool) {
	split := strings.Split(child.Id, supervisorName+"/")
	if len(split) == 2 && len(split[1]) > 0 {
		return split[1], true
	}
	return "", false
}

func (s *graphSupervisor) receiveEvent(context actor.Context) {
	switch msg := context.Message().(type) {

	case *model.GraphCreatedEvent:
		s.handleActiveGraph(msg.FlowId)

	case *model.GraphCompletedEvent:
		s.handleInactiveGraph(msg.FlowId)

	default:
		s.log.Infof("Ignoring replayed message of unknown type %v", reflect.TypeOf(msg))
	}
}

// this method will spawn a graph actor if it doesn't already exist
func (s *graphSupervisor) getGraphActor(context actor.Context, flowID string) (*actor.PID, error) {
	if child, ok := s.findChild(context, flowID); ok {
		return child, nil
	}
	return s.spawnGraphActor(context, flowID)
}

func (s *graphSupervisor) findChild(context actor.Context, flowID string) (*actor.PID, bool) {
	fullID := context.Self().Id + "/" + flowID
	for _, pid := range context.Children() {
		if pid.Id == fullID {
			return pid, true
		}
	}
	return nil, false
}

func (s *graphSupervisor) spawnGraphActor(context actor.Context, flowID string) (*actor.PID, error) {
	props := actor.
		FromInstance(NewGraphActor(s.executor)).
		WithMiddleware(
			plugin.Use(&PIDAwarePlugin{}),
			persistence.Using(s.persistenceProvider),
		)
	pid, err := context.SpawnNamed(props, flowID)
	if err != nil {
		return nil, err
	}
	context.Watch(pid)
	s.log.WithFields(logrus.Fields{"flow_id": flowID}).Infof("Created graph actor %s", pid.Id)
	return pid, nil
}
