package actor

import (
	"fmt"
	"net/url"
	"reflect"
	"time"

	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/eventstream"
	"github.com/fnproject/flow/blobs"
	"github.com/fnproject/flow/model"
	"github.com/fnproject/flow/persistence"
	"github.com/fnproject/flow/sharding"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"math"
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
	m.log.WithFields(logrus.Fields{"flow_id": req.FlowId}).Debug("Creating graph")
	r, e := m.forwardRequest(ctx, req)
	if e != nil {
		return nil, e
	}

	return r.(*model.CreateGraphResponse), e
}

func (m *actorManager) GetGraphState(ctx context.Context, req *model.GetGraphStateRequest) (*model.GetGraphStateResponse, error) {
	m.log.Debug("Getting graph stage")
	r, e := m.forwardRequest(ctx, req)
	if e != nil {
		return nil, e
	}
	return r.(*model.GetGraphStateResponse), e
}

func (m *actorManager) AddInvokeFunction(ctx context.Context, req *model.AddInvokeFunctionStageRequest) (*model.AddStageResponse, error) {
	m.log.Debug("Adding stage")
	r, e := m.forwardRequest(ctx, req)
	if e != nil {
		return nil, e
	}

	return r.(*model.AddStageResponse), e
}

func (m *actorManager) AddDelay(ctx context.Context, req *model.AddDelayStageRequest) (*model.AddStageResponse, error) {
	m.log.Debug("Adding stage")
	r, e := m.forwardRequest(ctx, req)
	if e != nil {
		return nil, e
	}

	return r.(*model.AddStageResponse), e
}

func (m *actorManager) AddStage(ctx context.Context, req *model.AddStageRequest) (*model.AddStageResponse, error) {
	r, e := m.forwardRequest(ctx, req)
	if e != nil {
		return nil, e
	}

	return r.(*model.AddStageResponse), e

}

func (m *actorManager) AddValueStage(ctx context.Context, req *model.AddCompletedValueStageRequest) (*model.AddStageResponse, error) {
	r, e := m.forwardRequest(ctx, req)
	if e != nil {
		return nil, e
	}

	return r.(*model.AddStageResponse), e

}

func (m *actorManager) AwaitStageResult(ctx context.Context, req *model.AwaitStageResultRequest) (*model.AwaitStageResultResponse, error) {
	m.log.WithFields(logrus.Fields{"flow_id": req.FlowId}).Debug("Getting stage result")
	if timeout := req.GetTimeoutMs(); timeout > 0 {
		const maxTimeoutEver = 1 * time.Hour
		awaitTimeout := time.Duration(math.Min(float64(time.Duration(timeout) * time.Millisecond), float64(maxTimeoutEver)))
		deadline, set := ctx.Deadline()
		if set {
			awaitTimeout = time.Duration(math.Min(float64(time.Until(deadline)), float64(awaitTimeout)))
		}
		ctx, _ = context.WithTimeout(ctx, awaitTimeout)
	}
	r, e := m.forwardRequest(ctx, req)
	if e != nil {
		return nil, e
	}

	return r.(*model.AwaitStageResultResponse), e
}

func (m *actorManager) CompleteStageExternally(ctx context.Context, req *model.CompleteStageExternallyRequest) (*model.CompleteStageExternallyResponse, error) {
	m.log.WithFields(logrus.Fields{"flow_id": req.FlowId}).Debug("Completing stage externally")
	r, e := m.forwardRequest(ctx, req)
	return r.(*model.CompleteStageExternallyResponse), e
}

func (m *actorManager) Commit(ctx context.Context, req *model.CommitGraphRequest) (*model.GraphRequestProcessedResponse, error) {
	m.log.WithFields(logrus.Fields{"flow_id": req.FlowId}).Debug("Committing graph")
	r, e := m.forwardRequest(ctx, req)
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
		return nil, status.Errorf(codes.Unimplemented, "Ignoring request of unknown type %v", reflect.TypeOf(req))
	}

	shardID, err := m.shardExtractor.ShardID(msg.GetFlowId())
	if err != nil {
		m.log.Warnf("Failed to extract shard for graph %s: %v", msg.GetFlowId(), err)
		return nil, model.NewGraphNotFoundError(msg.GetFlowId())
	}

	pid, ok := m.shardSupervisors[shardID]

	if !ok {
		m.log.Warnf("No local supervisor found for shard %d", shardID)
		return nil, model.NewGraphNotFoundError(msg.GetFlowId())
	}
	return pid, nil
}

func (m *actorManager) forwardRequest(ctx context.Context, req interface{}) (interface{}, error) {
	const defaultRequestTimeout = 5 * time.Second
	// Check http request
	var requestTimeout = defaultRequestTimeout
	deadline, set := ctx.Deadline()
	if set {
		requestTimeout = time.Until(deadline)
	}
	supervisor, err := m.lookupSupervisor(req)
	if err != nil {
		return nil, err
	}
	future := supervisor.RequestFuture(req, requestTimeout)
	r, err := future.Result()
	if err != nil {
		if err == actor.ErrTimeout {
			return nil, status.Error(codes.DeadlineExceeded, err.Error())
		}
		return nil, err
	}

	// Convert error responses back to errors
	if err, ok := r.(error); ok {
		return nil, err
	}
	return r, nil
}

func isLifecycleEvent(event *persistence.StreamEvent) (ok bool) {
	_, ok = event.Event.(*model.GraphLifecycleEvent)
	return
}

func (m *actorManager) StreamLifecycle(lr *model.StreamLifecycleRequest, stream model.FlowService_StreamLifecycleServer) error {
	m.log.Debug("Streaming lifecycle events")

	eventsChan := make(chan *model.GraphLifecycleEvent, 10)

	m.persistenceProvider.GetStreamingState().StreamNewEvents(isLifecycleEvent,
		func(event *persistence.StreamEvent) {
			eventsChan <- event.Event.(*model.GraphLifecycleEvent)
		})

	for event := range eventsChan {
		m.log.Debugf("Sent a graph lifecycle event %v", event)
		stream.Send(event)
	}
	return nil
}

func (m *actorManager) StreamEvents(gr *model.StreamGraphRequest, stream model.FlowService_StreamEventsServer) error {
	m.log.Debugf("Streaming graph events streamRequest= %T %+v", gr, gr)

	graphPath, err := m.graphActorPath(gr.FlowId)
	if err != nil {
		return err
	}

	eventsChan := make(chan *model.GraphEvent, 10)

	m.persistenceProvider.GetStreamingState().SubscribeActorJournal(graphPath, int(gr.FromSeq),
		func(event *persistence.StreamEvent) {
			if graphEvent, ok := event.Event.(*model.GraphEvent); ok {
				eventsChan <- graphEvent
			}
		})

	for event := range eventsChan {
		m.log.Debugf("Sent a graph event %v", event)
		stream.Send(event)
	}
	return nil
}

func (m *actorManager) StreamNewEvents(predicate persistence.StreamPredicate, fn persistence.StreamCallBack) *eventstream.Subscription {
	return m.persistenceProvider.GetStreamingState().StreamNewEvents(predicate, fn)
}

func (m *actorManager) SubscribeGraphEvents(flowID string, fromIndex int, fn persistence.StreamCallBack) (*eventstream.Subscription, error) {
	graphPath, err := m.graphActorPath(flowID)
	if err != nil {
		return nil, err
	}
	return m.persistenceProvider.GetStreamingState().SubscribeActorJournal(graphPath, fromIndex, fn), nil
}

func (m *actorManager) QueryGraphEvents(flowID string, fromIndex int, p persistence.StreamPredicate, fn persistence.StreamCallBack) error {
	graphPath, err := m.graphActorPath(flowID)
	if err != nil {
		return err
	}
	m.persistenceProvider.GetStreamingState().QueryActorJournal(graphPath, fromIndex, p, fn)
	return nil
}

func (m *actorManager) UnsubscribeStream(sub *eventstream.Subscription) {
	m.persistenceProvider.GetStreamingState().UnsubscribeStream(sub)
}

func (m *actorManager) graphActorPath(flowID string) (string, error) {
	shardID, err := m.shardExtractor.ShardID(flowID)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/%s", supervisorName(shardID), flowID), nil
}
