package actor

import (
	"fmt"
	"net/url"
	"reflect"
	"sync"
	"time"

	"github.com/fnproject/completer/sharding"

	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/eventstream"
	"github.com/fnproject/completer/model"
	"github.com/fnproject/completer/persistence"
	"github.com/sirupsen/logrus"
)

const (
	supervisorBaseName = "supervisor"
	unknownShardID     = -1
)

// GraphManager encapsulates all graph operations
type GraphManager interface {
	CreateGraph(*model.CreateGraphRequest, time.Duration) (*model.CreateGraphResponse, error)
	AddStage(model.AddStageCommand, time.Duration) (*model.AddStageResponse, error)
	GetStageResult(*model.GetStageResultRequest, time.Duration) (*model.GetStageResultResponse, error)
	CompleteStageExternally(*model.CompleteStageExternallyRequest, time.Duration) (*model.CompleteStageExternallyResponse, error)
	Commit(*model.CommitGraphRequest, time.Duration) (*model.GraphRequestProcessedResponse, error)
	GetGraphState(*model.GetGraphStateRequest, time.Duration) (*model.GetGraphStateResponse, error)
	StreamNewEvents(predicate persistence.StreamPredicate, fn persistence.StreamCallBack) *eventstream.Subscription
	SubscribeGraphEvents(graphID string, fromIndex int, fn persistence.StreamCallBack) (*eventstream.Subscription, error)
	QueryGraphEvents(graphID string, fromIndex int, p persistence.StreamPredicate, fn persistence.StreamCallBack) error
	UnsubscribeStream(sub *eventstream.Subscription)
}

type actorManager struct {
	log                 *logrus.Entry
	supervisorMtx       *sync.RWMutex
	shardedSupervisors  map[int]*actor.PID
	executor            *actor.PID
	persistenceProvider *persistence.StreamingProvider
	shardExtractor      sharding.ShardExtractor
	strategy            actor.SupervisorStrategy
}

// NewGraphManagerFromEnv creates a new implementation of the GraphManager interface
func NewGraphManager(persistenceProvider persistence.ProviderState, blobStore persistence.BlobStore, fnUrl string, shardExtractor sharding.ShardExtractor) (GraphManager, error) {

	log := logrus.WithField("logger", "graphmanager_actor")

	parsedUrl, err := url.Parse(fnUrl)
	if err != nil {
		return nil, fmt.Errorf("Invalid functions server URL: %s", err)
	}
	if parsedUrl.Path == "" {
		parsedUrl.Path = "r"
		fnUrl = parsedUrl.String()
	}

	decider := func(reason interface{}) actor.Directive {
		log.Warnf("Stopping failed graph actor due to error: %v", reason)
		return actor.StopDirective
	}
	strategy := actor.NewOneForOneStrategy(10, 1000, decider)

	executorProps := actor.FromInstance(NewExecutor(fnUrl, blobStore)).WithSupervisor(strategy)
	// TODO executor is not sharded and would not support clustering once it's made persistent!
	executor, _ := actor.SpawnNamed(executorProps, "executor")

	return &actorManager{
		log:                 log,
		supervisorMtx:       &sync.RWMutex{},
		shardedSupervisors:  make(map[int]*actor.PID),
		executor:            executor,
		persistenceProvider: persistence.NewStreamingProvider(persistenceProvider),
		shardExtractor:      shardExtractor,
		strategy:            strategy,
	}, nil
}

func (m *actorManager) QueryJournal(graphID string, eventIndex int, fn func(idx int, evt interface{})) {
	m.persistenceProvider.GetStreamingState().GetEvents(graphID, eventIndex, fn)
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

func (m *actorManager) Commit(req *model.CommitGraphRequest, timeout time.Duration) (*model.GraphRequestProcessedResponse, error) {
	m.log.WithFields(logrus.Fields{"graph_id": req.GraphId}).Debug("Committing graph")
	r, e := m.forwardRequest(req, timeout)
	if e != nil {
		return nil, e
	}
	return r.(*model.GraphRequestProcessedResponse), e
}

func supervisorName(shardID int) (name string) {
	if shardID == unknownShardID {
		name = supervisorBaseName
	} else {
		name = fmt.Sprintf("%s-%d", supervisorBaseName, shardID)
	}
	return
}

func (m *actorManager) newSupervisor(shardID int) (*actor.PID, error) {
	supervisorProps := actor.
		FromInstance(NewSupervisor(m.executor, m.persistenceProvider)).
		WithSupervisor(m.strategy).
		WithMiddleware(persistence.Using(m.persistenceProvider))

	return actor.SpawnNamed(supervisorProps, supervisorName(shardID))
}

func (m *actorManager) lookupShardedSupervisor(shardID int) (*actor.PID, error) {
	m.supervisorMtx.RLock()
	pid, ok := m.shardedSupervisors[shardID]
	m.supervisorMtx.RUnlock()
	if !ok {
		m.supervisorMtx.Lock()
		defer m.supervisorMtx.Unlock()
		var err error
		pid, err = m.newSupervisor(shardID)
		if err != nil {
			return nil, err
		}
		m.shardedSupervisors[shardID] = pid
	}
	return pid, nil
}

func (m *actorManager) lookupSupervisor(req interface{}) (*actor.PID, error) {
	msg, ok := req.(model.GraphMessage)
	if !ok {
		m.log.Warnf("Processing request of type %v for unknown shard", reflect.TypeOf(req))
		return m.lookupShardedSupervisor(unknownShardID)
	}
	shardID, err := m.shardExtractor.ShardID(msg.GetGraphId())
	if err != nil {
		return nil, model.NewGraphNotFoundError(msg.GetGraphId())
	}
	return m.lookupShardedSupervisor(shardID)
}

func (m *actorManager) forwardRequest(req interface{}, timeout time.Duration) (interface{}, error) {
	supervisor, err := m.lookupSupervisor(req)
	if err != nil {
		return nil, err
	}

	future := supervisor.RequestFuture(req, timeout)
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

func (m *actorManager) StreamNewEvents(predicate persistence.StreamPredicate, fn persistence.StreamCallBack) *eventstream.Subscription {
	return m.persistenceProvider.GetStreamingState().StreamNewEvents(predicate, fn)
}

func (m *actorManager) SubscribeGraphEvents(graphID string, fromIndex int, fn persistence.StreamCallBack) (*eventstream.Subscription, error) {
	graphPath, err := m.graphActorPath(graphID)
	if err != nil {
		return nil, err
	}
	return m.persistenceProvider.GetStreamingState().SubscribeActorJournal(graphPath, fromIndex, fn), nil
}

func (m *actorManager) QueryGraphEvents(graphID string, fromIndex int, p persistence.StreamPredicate, fn persistence.StreamCallBack) error {
	graphPath, err := m.graphActorPath(graphID)
	if err != nil {
		return err
	}
	m.persistenceProvider.GetStreamingState().QueryActorJournal(graphPath, fromIndex, p, fn)
	return nil
}

func (m *actorManager) UnsubscribeStream(sub *eventstream.Subscription) {
	m.persistenceProvider.GetStreamingState().UnsubscribeStream(sub)
}

func (m *actorManager) graphActorPath(graphID string) (string, error) {
	shardID, err := m.shardExtractor.ShardID(graphID)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/%s", supervisorName(shardID), graphID), nil
}
