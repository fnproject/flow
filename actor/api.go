package actor

import (
	"time"

	"github.com/AsynkronIT/protoactor-go/eventstream"

	"github.com/AsynkronIT/protoactor-go/actor"

	"github.com/fnproject/completer/model"
	"github.com/sirupsen/logrus"
)

// GraphManager encapsulates all graph operations
type GraphManager interface {
	CreateGraph(*model.CreateGraphRequest, time.Duration) *actor.Future
	AddStage(interface{}, time.Duration) *actor.Future
	GetStageResult(*model.GetStageResultRequest, time.Duration) *actor.Future
	CompleteStageExternally(*model.CompleteStageExternallyRequest, time.Duration) *actor.Future
	Commit(*model.CommitGraphRequest, time.Duration) *actor.Future
	SubscribeStream(graphID string, fn func(evt interface{})) *eventstream.Subscription
	UnsubscribeStream(sub *eventstream.Subscription)
	QueryJournal(graphID string, eventIndex int, fn func(evt interface{}))
}

type actorManager struct {
	log                 *logrus.Entry
	supervisor          *actor.PID
	executor            *actor.PID
	persistenceProvider *streamingInMemoryProvider
}

// NewGraphManager creates a new implementation of the GraphManager interface
func NewGraphManager(fnHost string, fnPort string) GraphManager {
	decider := func(reason interface{}) actor.Directive {
		log.Warnf("Graph actor child failed %v", reason)
		return actor.StopDirective
	}
	strategy := actor.NewOneForOneStrategy(10, 1000, decider)

	executorProps := actor.FromInstance(NewExecutor("http://" + fnHost + ":" + fnPort + "/r")).WithSupervisor(strategy)
	executor, _ := actor.SpawnNamed(executorProps, "executor")
	persistenceProvider := newStreamingInMemoryProvider(1000)

	supervisorProps := actor.FromInstance(NewSupervisor(executor, persistenceProvider)).WithSupervisor(strategy)
	supervisor, _ := actor.SpawnNamed(supervisorProps, "supervisor")

	return &actorManager{
		log:                 logrus.WithField("logger", "graphManager"),
		supervisor:          supervisor,
		executor:            executor,
		persistenceProvider: persistenceProvider,
	}
}

func (m *actorManager) SubscribeStream(graphID string, fn func(evt interface{})) *eventstream.Subscription {
	return m.persistenceProvider.GetEventStream().Subscribe(fn)
}

func (m *actorManager) UnsubscribeStream(sub *eventstream.Subscription) {
	m.persistenceProvider.GetEventStream().Unsubscribe(sub)
}

func (m *actorManager) QueryJournal(graphID string, eventIndex int, fn func(evt interface{})) {
	m.persistenceProvider.GetState().GetEvents(graphID, eventIndex, fn)
}

func (m *actorManager) CreateGraph(req *model.CreateGraphRequest, timeout time.Duration) *actor.Future {
	m.log.WithFields(logrus.Fields{"graph_id": req.GraphId}).Debug("Creating graph")
	return m.forwardRequest(req, timeout)
}
func (m *actorManager) AddStage(req interface{}, timeout time.Duration) *actor.Future {
	m.log.Debug("Adding stage")
	return m.forwardRequest(req, timeout)
}

func (m *actorManager) GetStageResult(req *model.GetStageResultRequest, timeout time.Duration) *actor.Future {
	m.log.WithFields(logrus.Fields{"graph_id": req.GraphId}).Debug("Getting stage result")
	return m.forwardRequest(req, timeout)
}

func (m *actorManager) CompleteStageExternally(req *model.CompleteStageExternallyRequest, timeout time.Duration) *actor.Future {
	m.log.WithFields(logrus.Fields{"graph_id": req.GraphId}).Debug("Completing stage externally")
	return m.forwardRequest(req, timeout)
}

func (m *actorManager) Commit(req *model.CommitGraphRequest, timeout time.Duration) *actor.Future {
	m.log.WithFields(logrus.Fields{"graph_id": req.GraphId}).Debug("Committing graph")
	return m.forwardRequest(req, timeout)
}

func (m *actorManager) forwardRequest(req interface{}, timeout time.Duration) *actor.Future {
	return m.supervisor.RequestFuture(req, timeout)
}
