package actor

import (
	"fmt"
	"net/url"
	"reflect"
	"github.com/fnproject/flow/sharding"

	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/eventstream"
	"github.com/fnproject/flow/blobs"
	"github.com/fnproject/flow/model"
	"github.com/fnproject/flow/persistence"
	"github.com/sirupsen/logrus"
	"context"
	"time"
)

const (
	supervisorBaseName = "supervisor"
)

type actorManager struct {
	log                 *logrus.Entry
	shardSupervisors    map[int]*actor.PID
	executor            *actor.PID
	persistenceProvider *persistence.StreamingProvider
	shardExtractor      sharding.ShardExtractor
	strategy            actor.SupervisorStrategy
}

// NewGraphManager creates a new implementation of the GraphManager interface
func NewGraphManager(persistenceProvider persistence.ProviderState, blobStore blobs.Store, fnURL string, shardExtractor sharding.ShardExtractor, shards []int) (model.FlowServiceServer, error) {

	log := logrus.WithField("logger", "graphmanager_actor")

	parsedURL, err := url.Parse(fnURL)
	if err != nil {
		return nil, fmt.Errorf("Invalid functions server URL: %s", err)
	}
	if parsedURL.Path == "" {
		parsedURL.Path = "r"
		fnURL = parsedURL.String()
	}

	decider := func(reason interface{}) actor.Directive {
		log.Warnf("Stopping failed graph actor due to error: %v", reason)
		return actor.StopDirective
	}
	strategy := actor.NewOneForOneStrategy(10, 1000, decider)

	executorProps := actor.FromInstance(NewExecutor(fnURL, blobStore)).WithSupervisor(strategy)
	// TODO executor is not sharded and would not support clustering once it's made persistent!
	executor, err := actor.SpawnNamed(executorProps, "executor")
	if err != nil {
		panic(fmt.Sprintf("Failed to spawn executor actor: %v", err))
	}

	streamingProvider := persistence.NewStreamingProvider(persistenceProvider)

	supervisors := make(map[int]*actor.PID)
	for _, shard := range shards {
		name := supervisorName(shard)
		supervisorProps := actor.
			FromInstance(NewSupervisor(name, executor, streamingProvider)).
			WithSupervisor(strategy).
			WithMiddleware(persistence.Using(streamingProvider))

		shardSupervisor, err := actor.SpawnNamed(supervisorProps, name)
		if err != nil {
			panic(fmt.Sprintf("Failed to spawn actor %s: %v", name, err))
		}
		supervisors[shard] = shardSupervisor
	}

	return &actorManager{
		log:                 log,
		shardSupervisors:    supervisors,
		executor:            executor,
		persistenceProvider: streamingProvider,
		shardExtractor:      shardExtractor,
		strategy:            strategy,
	}, nil
}

func (m *actorManager) CreateGraph(ctx context.Context, req *model.CreateGraphRequest) (*model.CreateGraphResponse, error) {
	m.log.WithFields(logrus.Fields{"graph_id": req.GraphId}).Debug("Creating graph")
	r, e := m.forwardRequest(req, ctx)
	if e != nil {
		return nil, e
	}

	return r.(*model.CreateGraphResponse), e
}

func (m *actorManager) GetGraphState(ctx context.Context, req *model.GetGraphStateRequest) (*model.GetGraphStateResponse, error) {
	m.log.Debug("Getting graph stage")
	r, e := m.forwardRequest(req, ctx)
	if e != nil {
		return nil, e
	}
	return r.(*model.GetGraphStateResponse), e
}

func (m *actorManager) AddInvokeFunction(ctx context.Context, req *model.AddInvokeFunctionStageRequest) (*model.AddStageResponse, error) {
	m.log.Debug("Adding stage")
	r, e := m.forwardRequest(req, ctx)
	if e != nil {
		return nil, e
	}

	return r.(*model.AddStageResponse), e
}

func (m *actorManager) AddDelay(ctx context.Context, req *model.AddDelayStageRequest) (*model.AddStageResponse, error) {
	m.log.Debug("Adding stage")
	r, e := m.forwardRequest(req, ctx)
	if e != nil {
		return nil, e
	}

	return r.(*model.AddStageResponse), e
}

func (m *actorManager) AddStage(ctx context.Context, req *model.AddStageRequest) (*model.AddStageResponse, error) {
	m.log.Debug("Adding stage")
	r, e := m.forwardRequest(req, ctx)
	if e != nil {
		return nil, e
	}

	return r.(*model.AddStageResponse), e

}

func (m *actorManager) AwaitStageResult(ctx context.Context, req *model.AwaitStageResultRequest) (*model.AwaitStageResultResponse, error) {
	m.log.WithFields(logrus.Fields{"graph_id": req.GraphId}).Debug("Getting stage result")
	r, e := m.forwardRequest(req, ctx)
	if e != nil {
		return nil, e
	}

	return r.(*model.AwaitStageResultResponse), e
}

func (m *actorManager) CompleteStageExternally(ctx context.Context, req *model.CompleteStageExternallyRequest) (*model.CompleteStageExternallyResponse, error) {
	m.log.WithFields(logrus.Fields{"graph_id": req.GraphId}).Debug("Completing stage externally")
	r, e := m.forwardRequest(req, ctx)
	return r.(*model.CompleteStageExternallyResponse), e
}

func (m *actorManager) Commit(ctx context.Context, req *model.CommitGraphRequest) (*model.GraphRequestProcessedResponse, error) {
	m.log.WithFields(logrus.Fields{"graph_id": req.GraphId}).Debug("Committing graph")
	r, e := m.forwardRequest(req, ctx)
	if e != nil {
		return nil, e
	}
	return r.(*model.GraphRequestProcessedResponse), e
}

func supervisorName(shardID int) (name string) {
	return fmt.Sprintf("%s-%d", supervisorBaseName, shardID)
}

func (m *actorManager) lookupSupervisor(req interface{}) (*actor.PID, error) {
	msg, ok := req.(model.GraphMessage)
	if !ok {
		return nil, fmt.Errorf("Ignoring request of unknown type %v", reflect.TypeOf(req))
	}

	shardID, err := m.shardExtractor.ShardID(msg.GetGraphId())
	if err != nil {
		m.log.Warnf("Failed to extract shard for graph %s: %v", msg.GetGraphId(), err)
		return nil, model.NewGraphNotFoundError(msg.GetGraphId())
	}

	pid, ok := m.shardSupervisors[shardID]

	if !ok {
		m.log.Warnf("No local supervisor found for shard %d", shardID)
		return nil, model.NewGraphNotFoundError(msg.GetGraphId())
	}
	return pid, nil
}

func (m *actorManager) forwardRequest(req interface{}, ctx context.Context) (interface{}, error) {
	supervisor, err := m.lookupSupervisor(req)
	if err != nil {
		return nil, err
	}
	// TODO: timeouts from context
	future := supervisor.RequestFuture(req, 5*time.Second)
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

func (m *actorManager) StreamEvents(req *model.StreamRequest, stream model.FlowService_StreamEventsServer) error {
	// TODO Hook up streams
	switch q := req.GetQuery().(type) {
	case model.StreamRequest_Lifecycle:
		{

		}
	case model.StreamRequest_Graph:
		{

		}
	}
	return nil
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
