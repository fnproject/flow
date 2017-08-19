package actor

import (
	"time"

	"github.com/AsynkronIT/protoactor-go/eventstream"

	"github.com/AsynkronIT/protoactor-go/actor"

	"github.com/fnproject/completer/model"
	"github.com/sirupsen/logrus"
	protoPersistence "github.com/AsynkronIT/protoactor-go/persistence"
	"net/url"
	"fmt"
	"github.com/fnproject/completer/setup"
	"github.com/fnproject/completer/persistence"
)

// GraphManager encapsulates all graph operations
type GraphManager interface {
	CreateGraph(*model.CreateGraphRequest, time.Duration) (*model.CreateGraphResponse, error)
	AddStage(model.AddStageCommand, time.Duration) (*model.AddStageResponse, error)
	GetStageResult(*model.GetStageResultRequest, time.Duration) (*model.GetStageResultResponse, error)
	CompleteStageExternally(*model.CompleteStageExternallyRequest, time.Duration) (*model.CompleteStageExternallyResponse, error)
	Commit(*model.CommitGraphRequest, time.Duration) (*model.CommitGraphProcessed, error)
	GetGraphState( *model.GetGraphStateRequest, time.Duration) (*model.GetGraphStateResponse, error)
	SubscribeStream(graphID string, fn func(evt interface{})) *eventstream.Subscription
	UnsubscribeStream(sub *eventstream.Subscription)
	QueryJournal(graphID string, eventIndex int, fn func(evt interface{}))
}

type actorManager struct {
	log                 *logrus.Entry
	supervisor          *actor.PID
	executor            *actor.PID
	persistenceProvider *persistence.StreamingProvider
}

// NewGraphManagerFromEnv creates a new implementation of the GraphManager interface
func NewGraphManagerFromEnv(persistenceProvider protoPersistence.ProviderState) (GraphManager, error) {
	fnUrl := setup.GetString(setup.EnvFnApiURL)

	log := logrus.WithField("logger", "graphmanager_actor")
	decider := func(reason interface{}) actor.Directive {
		log.Warnf("Graph actor child failed %v", reason)
		return actor.StopDirective
	}
	strategy := actor.NewOneForOneStrategy(10, 1000, decider)

	parsedUrl, err := url.Parse(fnUrl)
	if err != nil {
		return nil, fmt.Errorf("Invalid functions server URL: %s", err)
	}
	if parsedUrl.Path == "" {
		parsedUrl.Path = "r"
		fnUrl = parsedUrl.String()
	}

	executorProps := actor.FromInstance(NewExecutor(fnUrl)).WithSupervisor(strategy)
	executor, _ := actor.SpawnNamed(executorProps, "executor")
	wrappedProvider := persistence.NewStreamingProvider(persistenceProvider)

	supervisorProps := actor.FromInstance(NewSupervisor(executor, wrappedProvider)).WithSupervisor(strategy)
	supervisor, _ := actor.SpawnNamed(supervisorProps, "supervisor")

	return &actorManager{
		log:        log,
		supervisor: supervisor,
		executor:   executor, persistenceProvider: wrappedProvider,
	}, nil
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

func (m *actorManager) CreateGraph(req *model.CreateGraphRequest, timeout time.Duration) (*model.CreateGraphResponse, error) {
	m.log.WithFields(logrus.Fields{"graph_id": req.GraphId}).Debug("Creating graph")
	r, e := m.forwardRequest(req, timeout)
	if e != nil {
		return nil, e
	}

	return r.(*model.CreateGraphResponse), e
}

func (m *actorManager) GetGraphState(req *model.GetGraphStateRequest, timeout time.Duration) (*model.GetGraphStateResponse, error) {
	m.log.Debug("Getting graph stage")
	r, e := m.forwardRequest(req, timeout)
	if e != nil {
		return nil, e
	}
	return r.(*model.GetGraphStateResponse), e
}
func (m *actorManager) AddStage(req model.AddStageCommand, timeout time.Duration) (*model.AddStageResponse, error) {
	m.log.Debug("Adding stage")
	r, e := m.forwardRequest(req, timeout)
	if e != nil {
		return nil, e
	}

	return r.(*model.AddStageResponse), e

}

func (m *actorManager) GetStageResult(req *model.GetStageResultRequest, timeout time.Duration) (*model.GetStageResultResponse, error) {
	m.log.WithFields(logrus.Fields{"graph_id": req.GraphId}).Debug("Getting stage result")
	r, e := m.forwardRequest(req, timeout)
	if e != nil {
		return nil, e
	}

	return r.(*model.GetStageResultResponse), e
}

func (m *actorManager) CompleteStageExternally(req *model.CompleteStageExternallyRequest, timeout time.Duration) (*model.CompleteStageExternallyResponse, error) {
	m.log.WithFields(logrus.Fields{"graph_id": req.GraphId}).Debug("Completing stage externally")
	r, e := m.forwardRequest(req, timeout)
	return r.(*model.CompleteStageExternallyResponse), e
}

func (m *actorManager) Commit(req *model.CommitGraphRequest, timeout time.Duration) (*model.CommitGraphProcessed, error) {
	m.log.WithFields(logrus.Fields{"graph_id": req.GraphId}).Debug("Committing graph")
	r, e := m.forwardRequest(req, timeout)
	if e != nil {
		return nil, e
	}
	return r.(*model.CommitGraphProcessed), e
}

func (m *actorManager) forwardRequest(req interface{}, timeout time.Duration) (interface{}, error) {

	future := m.supervisor.RequestFuture(req, timeout)
	r, err := future.Result()
	if err != nil {
		return nil, err
	}

	// Convert error responses back to errors
	if err, ok := r.(error); ok {
		return nil, err
	}

	return r, nil
}
