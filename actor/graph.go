package actor

import (
	"reflect"
	"time"

	"github.com/AsynkronIT/protoactor-go/actor"
	protoPersistence "github.com/AsynkronIT/protoactor-go/persistence"
	"github.com/fnproject/completer/graph"
	"github.com/fnproject/completer/model"
	"github.com/fnproject/completer/persistence"
	"github.com/sirupsen/logrus"
	"github.com/golang/protobuf/ptypes/timestamp"
)

// TODO: read this from configuration!
const (
	maxDelaySeconds = 900
	inactiveTimeout = time.Duration(24 * time.Hour)
)

type graphActor struct {
	PIDHolder
	graph    *graph.CompletionGraph
	log      *logrus.Entry
	executor *actor.PID
	persistence.Mixin
}

// NewGraphActor returns a pointer to a new graph actor
func NewGraphActor(executor *actor.PID) actor.Actor {
	return &graphActor{
		executor: executor,
		log:      logrus.New().WithField("logger", "graph_actor"),
	}
}

func (g *graphActor) applyDelayScheduledEvent(event *model.DelayScheduledEvent) {
	// we always need to complete delay nodes from scratch to avoid completing twice
	delayMs := event.TimeMs - timeMillis()
	if delayMs > 0 {
		g.log.WithFields(logrus.Fields{"stage_id": event.StageId}).Debug("Scheduling delayed completion of stage")
		// Wait for the delay in a goroutine so we can complete the request in the meantime
		go func() {
			<-time.After(time.Duration(delayMs) * time.Millisecond)
			g.pid.Tell(&model.CompleteDelayStageRequest{
				GraphId: g.graph.ID,
				StageId: event.StageId,
				Result:  model.NewSuccessfulResult(model.NewEmptyDatum()),
			})
		}()
	} else {
		g.log.WithFields(logrus.Fields{"stage_id": event.StageId}).Debug("Queuing completion of delayed stage")
		g.pid.Tell(&model.CompleteDelayStageRequest{
			GraphId: g.graph.ID,
			StageId: event.StageId,
			Result:  model.NewSuccessfulResult(model.NewEmptyDatum()),
		})
	}
}

func timeMillis() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}

func (g *graphActor) initGraph(event *model.GraphCreatedEvent) {
	if g.graph != nil {
		g.log.Warn("Graph is already initialized!")
		return
	}
	g.log = g.log.WithFields(logrus.Fields{"logger": "graph_actor", "graph_id": event.GraphId, "function_id": event.FunctionId})
	g.graph = graph.New(event.GraphId, event.FunctionId, g)
}

func (g *graphActor) receiveEvent(event model.Event) {

	switch e := event.(type) {

	case *model.GraphCreatedEvent:
		g.initGraph(e)

	default:
		g.updateState(e)
	}
}

// Validate a list of stages. If any of them is missing, returns false and the first stage which is missing.
func (g *graphActor) validateStages(stageIDs []string) (bool, string) {
	for _, stage := range stageIDs {
		if g.graph.GetStage(stage) == nil {
			return false, stage
		}
	}
	return true, ""
}

// if validation fails, this method will respond to the request with an appropriate error message
func (g *graphActor) validateCmd(cmd interface{}, context actor.Context) bool {
	// skip validation for initial create
	if _, ok := cmd.(*model.CreateGraphRequest); ok {
		return true
	}

	// First check the graph exists
	if gm, ok := cmd.(model.GraphMessage); ok {
		graphID := gm.GetGraphId()
		if g.graph == nil {
			context.Respond(NewGraphNotFoundError(graphID))
			return false
		}

		// disallow graph structural changes when complete
		switch msg := gm.(type) {
		case *model.AddChainedStageRequest, *model.AddCompletedValueStageRequest, *model.AddDelayStageRequest, *model.AddExternalCompletionStageRequest, *model.AddInvokeFunctionStageRequest, *model.CompleteStageExternallyRequest:
			if g.graph.IsCompleted() {
				context.Respond(NewGraphCompletedError(msg.(model.GraphMessage).GetGraphId()))
				return false
			}
		}
	}

	// Then do individual checks dependent on type
	switch msg := cmd.(type) {
	case *model.AddDelayStageRequest:
		if msg.DelayMs <= 0 || msg.DelayMs > maxDelaySeconds*1000 {
			context.Respond(NewInvalidDelayError(msg.GraphId, msg.DelayMs))
			return false
		}

	case *model.AddChainedStageRequest:
		valid, missing := g.validateStages(msg.Deps)
		if !valid {
			context.Respond(NewStageNotFoundError(msg.GraphId, missing))
			return false
		}

	case *model.CompleteDelayStageRequest:
		if g.graph.IsCompleted() {
			// ignore internal CompleteDelaysStageRequest messages
			// to avoid accumulating duplicate StageCompletedEvents
			return false
		}

	case *model.CompleteStageExternallyRequest:
		stage := g.graph.GetStage(msg.StageId)
		if stage == nil {
			context.Respond(NewStageNotFoundError(msg.GraphId, msg.StageId))
			return false
		}
		if stage.GetOperation() != model.CompletionOperation_externalCompletion {
			g.log.WithFields(logrus.Fields{"stage_id": msg.StageId}).Debug("Stage is not externally completable")
			context.Respond(NewStageNotCompletableError(msg.GraphId, msg.StageId))
			return false
		}

	case *model.GetStageResultRequest:
		valid, missing := g.validateStages(append(make([]string, 0), msg.StageId))
		if !valid {
			context.Respond(NewStageNotFoundError(msg.GraphId, missing))
			return false
		}

	case *model.CommitGraphRequest:
		if g.graph.IsCompleted() || g.graph.IsCommitted() {
			context.Respond(&model.CommitGraphProcessed{GraphId: msg.GraphId})
			return false
		}
	}

	return true
}

func currentTimestamp() *timestamp.Timestamp {
	now := time.Now()

	return &timestamp.Timestamp{
		Seconds: now.Unix(),
		Nanos:   int32(now.Nanosecond()),
	}
}
func (g *graphActor) updateState(event interface{}) {
	if g.graph == nil {
		g.log.Warnf("Ignoring state update for event %v since graph is not initialized", reflect.TypeOf(event))
		return
	}
	g.graph.UpdateWithEvent(event, !g.Recovering())
}

func (g *graphActor) receiveCommand(context actor.Context) {
	if !g.validateCmd(context.Message(), context) {
		return
	}

	switch msg := context.Message().(type) {

	case *model.CreateGraphRequest:
		g.log.Debug("Creating graph")
		event := &model.GraphCreatedEvent{GraphId: msg.GraphId, FunctionId: msg.FunctionId, Ts: currentTimestamp()}
		g.PersistReceive(event)
		g.initGraph(event)
		context.Respond(&model.CreateGraphResponse{GraphId: msg.GraphId})

	case *model.GetGraphStateRequest:
		g.log.Debug("Get graph state")
		context.Respond(g.createExternalState())

	case *model.AddChainedStageRequest:
		g.log.Debug("Adding chained stage")
		event := &model.StageAddedEvent{
			StageId:      g.graph.NextStageID(),
			Op:           msg.Operation,
			Closure:      msg.Closure,
			Dependencies: msg.Deps,
			Ts:           currentTimestamp(),
		}
		g.PersistReceive(event)
		g.updateState(event)
		context.Respond(&model.AddStageResponse{GraphId: msg.GraphId, StageId: event.StageId})

	case *model.AddCompletedValueStageRequest:
		g.log.Debug("Adding completed value stage")
		addedEvent := &model.StageAddedEvent{
			StageId: g.graph.NextStageID(),
			Op:      msg.GetOperation(),
			Ts:      currentTimestamp(),
		}
		g.PersistReceive(addedEvent)
		g.updateState(addedEvent)
		completedEvent := &model.StageCompletedEvent{
			StageId: addedEvent.StageId,
			Result:  msg.Result,
			Ts:      currentTimestamp(),
		}
		g.PersistReceive(completedEvent)
		g.updateState(completedEvent)
		context.Respond(&model.AddStageResponse{GraphId: msg.GraphId, StageId: addedEvent.StageId})

	case *model.AddDelayStageRequest:
		g.log.Debug("Adding delay stage")
		addedEvent := &model.StageAddedEvent{
			StageId: g.graph.NextStageID(),
			Op:      msg.GetOperation(),
			Ts:      currentTimestamp(),
		}
		g.PersistReceive(addedEvent)
		g.updateState(addedEvent)
		delayEvent := &model.DelayScheduledEvent{
			StageId: addedEvent.StageId,
			TimeMs:  timeMillis() + msg.DelayMs,
			Ts:      currentTimestamp(),
		}
		g.PersistReceive(delayEvent)
		g.applyDelayScheduledEvent(delayEvent)
		context.Respond(&model.AddStageResponse{GraphId: msg.GraphId, StageId: addedEvent.StageId})

	case *model.AddExternalCompletionStageRequest:
		g.log.Debug("Adding external completion stage")
		event := &model.StageAddedEvent{
			StageId: g.graph.NextStageID(),
			Op:      msg.GetOperation(),
			Ts:      currentTimestamp(),
		}
		g.PersistReceive(event)
		g.updateState(event)
		context.Respond(&model.AddStageResponse{GraphId: msg.GraphId, StageId: event.StageId})

	case *model.AddInvokeFunctionStageRequest:
		g.log.Debug("Adding invoke stage")
		event := &model.StageAddedEvent{
			StageId: g.graph.NextStageID(),
			Op:      msg.GetOperation(),
			Ts:      currentTimestamp(),
		}
		g.PersistReceive(event)
		g.updateState(event)
		req := &model.InvokeFunctionRequest{
			GraphId:    g.graph.ID,
			StageId:    event.StageId,
			FunctionId: msg.FunctionId,
			Arg:        msg.Arg,
		}
		g.executor.Request(req, g.GetSelf())
		context.Respond(&model.AddStageResponse{GraphId: msg.GraphId, StageId: event.StageId})

	case *model.CompleteStageExternallyRequest:
		g.log.WithFields(logrus.Fields{"stage_id": msg.StageId}).Debug("Completing stage externally")
		stage := g.graph.GetStage(msg.StageId)
		completable := !stage.IsResolved()
		if completable {
			completedEvent := &model.StageCompletedEvent{
				StageId: msg.StageId,
				Result:  msg.Result,
				Ts:      currentTimestamp(),
			}
			g.PersistReceive(completedEvent)
			g.updateState(completedEvent)
		}
		context.Respond(&model.CompleteStageExternallyResponse{GraphId: msg.GraphId, StageId: msg.StageId, Successful: completable})

	case *model.CommitGraphRequest:
		g.log.Debug("Committing graph")
		committedEvent := &model.GraphCommittedEvent{GraphId: msg.GraphId, Ts: currentTimestamp()}
		g.PersistReceive(committedEvent)
		g.updateState(committedEvent)
		context.Respond(&model.CommitGraphProcessed{GraphId: msg.GraphId})

	case *model.GetStageResultRequest:
		g.log.WithFields(logrus.Fields{"stage_id": msg.StageId}).Debug("Retrieving stage result")
		stage := g.graph.GetStage(msg.StageId)
		context.AwaitFuture(stage.WhenComplete(), func(result interface{}, err error) {
			if err != nil {
				context.Respond(NewStageCompletionError(msg.GraphId, msg.StageId))
				return
			}
			response := &model.GetStageResultResponse{
				GraphId: msg.GraphId,
				StageId: msg.StageId,
				Result:  result.(*model.CompletionResult),
			}
			context.Respond(response)
		})

	case *model.CompleteDelayStageRequest:
		g.log.WithFields(logrus.Fields{"stage_id": msg.StageId}).Debug("Completing delayed stage")
		completedEvent := &model.StageCompletedEvent{
			StageId: msg.StageId,
			Result:  msg.Result,
			Ts:      currentTimestamp(),
		}
		g.PersistReceive(completedEvent)
		g.updateState(completedEvent)

	case *model.FaasInvocationResponse:
		g.log.WithFields(logrus.Fields{"stage_id": msg.StageId}).Debug("Received fn invocation response")
		completedEvent := &model.FaasInvocationCompletedEvent{
			StageId: msg.StageId,
			Result:  msg.Result,
			Ts:      currentTimestamp(),
		}
		g.PersistReceive(completedEvent)
		g.updateState(completedEvent)

	case *model.DeactivateGraphRequest:
		g.log.Debug("Telling supervisor graph is completed")
		// tell supervisor to remove us from active graphs
		context.Parent().Tell(msg)

	case *protoPersistence.RequestSnapshot:
		// snapshots are currently not supported
		// NOOP
		g.log.Debug("Ignoring snapshot request")

	case *actor.Started:
		g.log.Debugf("Started actor %s", g.GetSelf().Id)
		context.SetReceiveTimeout(inactiveTimeout)

	case *actor.ReceiveTimeout:
		g.log.Debugf("Passivating inactive actor %s", g.GetSelf().Id)
		if g.graph != nil {
			// tell supervisor to remove us from active graphs
			context.Parent().Tell(&model.DeactivateGraphRequest{GraphId: g.graph.ID})
		}

	case *protoPersistence.ReplayComplete:
		if g.graph != nil {
			g.log.Debug("Replay completed")
			g.graph.Recover()

			if g.graph.IsCompleted() {
				// tell supervisor to remove us from active graphs
				context.Parent().Tell(&model.DeactivateGraphRequest{GraphId: g.graph.ID})
			}
		}

	default:
		g.log.Debugf("Ignoring message of unknown type %v", reflect.TypeOf(msg))
	}
}

func (g *graphActor) createExternalState() *model.GetGraphStateResponse {
	stageOut := make(map[string]*model.GetGraphStateResponse_StageRepresentation)
	for _, s := range g.graph.GetStages() {
		var status string
		if s.IsFailed() {
			status = "failed"
		} else if s.IsSuccessful() {
			status = "successful"
		} else if s.IsTriggered() {
			status = "running"
		} else {
			status = "pending"
		}

		stageDeps := s.GetDeps()
		deps := make([]string, len(stageDeps))
		for i, dep := range stageDeps {
			deps[i] = dep.ID
		}

		rep := &model.GetGraphStateResponse_StageRepresentation{
			Type:         model.CompletionOperation_name[int32(s.GetOperation())],
			Status:       status,
			Dependencies: deps,
		}
		stageOut[s.ID] = rep
	}
	return &model.GetGraphStateResponse{
		GraphId:    g.graph.ID,
		FunctionId: g.graph.FunctionID,
		Stages:     stageOut,
	}
}

func (g *graphActor) Receive(context actor.Context) {
	g.log.Debugf("Processing message %s (recovering=%v)", reflect.TypeOf(context.Message()), g.Recovering())
	if g.Recovering() {
		if e,ok := context.Message().(model.Event) ; ok {
			g.receiveEvent(e)
		}
	} else {
		g.receiveCommand(context)
	}
}

func (g *graphActor) OnExecuteStage(stage *graph.CompletionStage, datum []*model.Datum) {
	g.log.WithField("stage_id", stage.ID).Info("Executing Stage")
	msg := &model.InvokeStageRequest{
		FunctionId: g.graph.FunctionID,
		GraphId:    g.graph.ID,
		StageId:    stage.ID,
		Args:       datum,
		Closure:    stage.GetClosure(),
		Operation:  stage.GetOperation(),
	}
	g.executor.Request(msg, g.GetSelf())
}

//OnCompleteStage indicates that a stage is finished and its result is available
func (g *graphActor) OnCompleteStage(stage *graph.CompletionStage, result *model.CompletionResult) {
	g.log.WithField("stage_id", stage.ID).Info("Completing stage in OnCompleteStage")
	completedEvent := &model.StageCompletedEvent{
		StageId: stage.ID,
		Result:  result,
		Ts:      currentTimestamp(),
	}
	g.PersistReceive(completedEvent)
	g.updateState(completedEvent)
}

//OnCompose Stage indicates that another stage should be composed into this one
func (g *graphActor) OnComposeStage(stage *graph.CompletionStage, composedStage *graph.CompletionStage) {
	g.log.WithField("stage_id", stage.ID).Info("Composing stage in OnComposeStage")
	composedEvent := &model.StageComposedEvent{
		StageId:         stage.ID,
		ComposedStageId: composedStage.ID,
		Ts:              currentTimestamp(),
	}
	g.PersistReceive(composedEvent)
	g.updateState(composedEvent)
}

//OnCompleteGraph indicates that the graph is now finished and cannot be modified
func (g *graphActor) OnCompleteGraph() {
	if g.Recovering() {
		return
	}
	g.log.Info("Completing graph in OnCompleteGraph")
	completedEvent := &model.GraphCompletedEvent{
		GraphId:    g.graph.ID,
		FunctionId: g.graph.FunctionID,
		Ts:         currentTimestamp(),
	}
	g.PersistReceive(completedEvent)
	g.updateState(completedEvent)
	g.GetSelf().Tell(&model.DeactivateGraphRequest{GraphId: g.graph.ID})
}
