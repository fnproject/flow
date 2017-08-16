package actor

import (
	"reflect"
	"time"

	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/persistence"
	"github.com/fnproject/completer/graph"
	"github.com/fnproject/completer/model"
	proto "github.com/golang/protobuf/proto"
	"github.com/sirupsen/logrus"
)

type graphActor struct {
	PIDHolder
	graph    *graph.CompletionGraph
	log      *logrus.Entry
	executor *actor.PID
	persistence.Mixin
}

func NewGraphActor(graphId string, functionId string, executor *actor.PID) *graphActor {
	return &graphActor{
		executor: executor,
		log:      logrus.WithFields(logrus.Fields{"logger": "graph_actor", "graph_id": graphId, "function_id": functionId}),
	}
}

func (g *graphActor) persist(event proto.Message) {
	g.PersistReceive(event)
}

func (g *graphActor) applyGraphCreatedEvent(event *model.GraphCreatedEvent) {
	g.log.Debug("Creating completion graph")
	g.graph = graph.New(event.GraphId, event.FunctionId, g)
}

func (g *graphActor) applyGraphCommittedEvent(event *model.GraphCommittedEvent) {
	g.log.Debug("Committing graph")
	g.graph.HandleCommitted()
}

func (g *graphActor) applyGraphCompletedEvent(event *model.GraphCompletedEvent) {
	g.log.Debug("Completing graph")
	g.graph.HandleCompleted()
	// "poison pill"
	g.pid.Stop()
}

func (g *graphActor) applyStageAddedEvent(event *model.StageAddedEvent) {
	g.log.WithFields(logrus.Fields{"stage_id": event.StageId}).Debug("Adding stage")
	g.graph.HandleStageAdded(event, !g.Recovering())
}

func (g *graphActor) applyStageCompletedEvent(event *model.StageCompletedEvent) {
	g.log.WithFields(logrus.Fields{"stage_id": event.StageId}).Debug("Completing stage")
	g.graph.HandleStageCompleted(event, !g.Recovering())
}

func (g *graphActor) applyStageComposedEvent(event *model.StageComposedEvent) {
	g.log.WithFields(logrus.Fields{"stage_id": event.StageId}).Debug("Composing stage")
	g.graph.HandleStageComposed(event)
}

func (g *graphActor) applyDelayScheduledEvent(event *model.DelayScheduledEvent) {
	// we always need to complete delay nodes from scratch to avoid completing twice
	delayMs := int64(event.DelayedTs) - timeMillis()
	if delayMs > 0 {
		g.log.WithFields(logrus.Fields{"stage_id": event.StageId}).Debug("Scheduling delayed completion of stage")
		// Wait for the delay in a goroutine so we can complete the request in the meantime
		go func() {
			<-time.After(time.Duration(delayMs) * time.Millisecond)
			g.pid.Tell(model.CompleteDelayStageRequest{
				string(g.graph.ID),
				event.StageId,
				model.NewSuccessfulResult(&model.Datum{Val: &model.Datum_Empty{Empty: &model.EmptyDatum{}}})})
		}()
	} else {
		g.log.WithFields(logrus.Fields{"stage_id": event.StageId}).Debug("Queuing completion of delayed stage")
		g.pid.Tell(model.CompleteDelayStageRequest{
			string(g.graph.ID),
			event.StageId,
			model.NewSuccessfulResult(&model.Datum{Val: &model.Datum_Empty{Empty: &model.EmptyDatum{}}})})
	}
}

func timeMillis() int64 {
	return time.Now().UnixNano() / 1000000
}

// TODO: read this from configuration!
const maxDelaySeconds = 900

func (g *graphActor) applyNoop(event interface{}) {

}

// process events
func (g *graphActor) receiveRecover(context actor.Context) {
}

// Validate a list of stages. If any of them is missing, returns false and the first stage which is missing.
func (g *graphActor) validateStages(stageIDs []string) (bool, string) {
	for _,stage := range stageIDs {
		if g.graph.GetStage(stage) == nil {
			return false, stage
		}
	}
	return true, ""
}

// if validation fails, this method will respond to the request with an appropriate error message
func (g *graphActor) validateCmd(cmd interface{}, context actor.Context) bool {
	// First check the graph exists
	if isGraphMessage(cmd) {
		graphId := getGraphId(cmd)
		if g.graph == nil {
			context.Respond(NewGraphNotFoundError(graphId))
			return false
		}
	}

	// Then do individual checks dependent on type
	switch msg := cmd.(type) {
	case *model.AddDelayStageRequest:
		if g.graph.IsCompleted() {
			context.Respond(NewGraphCompletedError(msg.GraphId))
			return false
		}
		if msg.DelayMs <= 0 || msg.DelayMs > maxDelaySeconds * 1000 {
			context.Respond(NewInvalidDelayError(msg.GraphId, msg.DelayMs))
			return false
		}

	case *model.AddChainedStageRequest:
		if g.graph.IsCompleted() {
			context.Respond(NewGraphCompletedError(msg.GraphId))
			return false
		}
		valid, missing := g.validateStages(msg.Deps)
		if !valid {
			context.Respond(NewStageNotFoundError(msg.GraphId, missing))
			return false
		}

    case *model.CompleteDelayStageRequest:
		if g.graph.IsCompleted() {
			// Don't respond, just ignore this message. This is intentional.
			return false
		}

	case *model.CompleteStageExternallyRequest:
		if g.graph.IsCompleted() {
			context.Respond(NewGraphCompletedError(msg.GraphId))
			return false
		}
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

// process commands
func (g *graphActor) receiveStandard(context actor.Context) {
	switch msg := context.Message().(type) {

	case *model.CreateGraphRequest:
		g.log.Debug("Creating graph")
		event := &model.GraphCreatedEvent{GraphId: msg.GraphId, FunctionId: msg.FunctionId}
		g.persist(event)
		g.applyGraphCreatedEvent(event)
		context.Respond(&model.CreateGraphResponse{GraphId: msg.GraphId})

	case *model.AddChainedStageRequest:
		if !g.validateCmd(msg, context) {
			return
		}
		g.log.Debug("Adding chained stage")
		event := &model.StageAddedEvent{
			StageId:      g.graph.NextStageID(),
			Op:           msg.Operation,
			Closure:      msg.Closure,
			Dependencies: msg.Deps,
		}
		g.persist(event)
		g.applyStageAddedEvent(event)
		context.Respond(&model.AddStageResponse{GraphId: msg.GraphId, StageId: event.StageId})

	case *model.AddCompletedValueStageRequest:
		if !g.validateCmd(msg, context) {
			return
		}
		g.log.Debug("Adding completed value stage")

		addedEvent := &model.StageAddedEvent{
			StageId: g.graph.NextStageID(),
			Op:      model.CompletionOperation_completedValue,
		}
		g.persist(addedEvent)
		g.applyStageAddedEvent(addedEvent)

		completedEvent := &model.StageCompletedEvent{
			StageId: g.graph.NextStageID(),
			Result:  msg.Result,
		}
		g.persist(completedEvent)
		g.applyStageCompletedEvent(completedEvent)
		context.Respond(&model.AddStageResponse{GraphId: msg.GraphId, StageId: addedEvent.StageId})

	case *model.AddDelayStageRequest:
		if !g.validateCmd(msg, context) {
			return
		}
		g.log.Debug("Adding delay stage")

		addedEvent := &model.StageAddedEvent{
			StageId: g.graph.NextStageID(),
			Op:      model.CompletionOperation_delay,
		}
		g.persist(addedEvent)
		g.applyStageAddedEvent(addedEvent)

		delayEvent := &model.DelayScheduledEvent{
			StageId:   g.graph.NextStageID(),
			DelayedTs: uint64(timeMillis()) + msg.DelayMs,
		}
		g.persist(delayEvent)
		g.applyDelayScheduledEvent(delayEvent)
		context.Respond(&model.AddStageResponse{GraphId: msg.GraphId, StageId: addedEvent.StageId})

	case *model.AddExternalCompletionStageRequest:
		if !g.validateCmd(msg, context) {
			return
		}
		g.log.Debug("Adding external completion stage")
		event := &model.StageAddedEvent{
			StageId: g.graph.NextStageID(),
			Op:      model.CompletionOperation_externalCompletion,
		}
		g.persist(event)
		g.applyStageAddedEvent(event)
		context.Respond(&model.AddStageResponse{GraphId: msg.GraphId, StageId: event.StageId})

	case *model.AddInvokeFunctionStageRequest:
		if !g.validateCmd(msg, context) {
			return
		}
		g.log.Debug("Adding invoke stage")

		event := &model.StageAddedEvent{
			StageId: g.graph.NextStageID(),
			Op:      model.CompletionOperation_completedValue,
		}
		g.persist(event)
		g.applyStageAddedEvent(event)

		/* TODO graph executor
		invokeReq := &model.InvokeFunctionRequest{
			GraphId:    msg.GraphId,
			StageId:    event.StageId,
			FunctionId: msg.FunctionId,
			Arg:        msg.Arg,
		}
		executor.Request(invokeReq)
		*/

		context.Respond(&model.AddStageResponse{GraphId: msg.GraphId, StageId: event.StageId})

	case *model.CompleteStageExternallyRequest:
		g.log.WithFields(logrus.Fields{"stage_id": msg.StageId}).Debug("Completing stage externally")
		if !g.validateCmd(msg, context) {
			return
		}
		stage := g.graph.GetStage(msg.StageId)
		completable := !stage.IsResolved()
		if completable {
			completedEvent := &model.StageCompletedEvent{
				StageId: msg.StageId,
				Result:  msg.Result,
			}
			g.persist(completedEvent)
			g.applyStageCompletedEvent(completedEvent)

		}
		context.Respond(&model.CompleteStageExternallyResponse{GraphId: msg.GraphId, StageId: msg.StageId, Successful: completable})

	case *model.CommitGraphRequest:
		g.log.Debug("Committing graph")
		if !g.validateCmd(msg, context) {
			return
		}
		committedEvent := &model.GraphCommittedEvent{GraphId: msg.GraphId}
		g.persist(committedEvent)
		g.applyGraphCommittedEvent(committedEvent)
		context.Respond(&model.CommitGraphProcessed{GraphId: msg.GraphId})

	case *model.GetStageResultRequest:
		g.log.WithFields(logrus.Fields{"stage_id": msg.StageId}).Debug("Retrieving stage result")
		datum := &model.Datum{
			Val: &model.Datum_Blob{
				Blob: &model.BlobDatum{ContentType: "text", DataString: []byte("foo")},
			},
		}
		result := &model.CompletionResult{Successful: true, Datum: datum}
		context.Respond(&model.GetStageResultResponse{GraphId: msg.GraphId, StageId: msg.StageId, Result: result})

	case *model.CompleteDelayStageRequest:
		g.log.WithFields(logrus.Fields{"stage_id": msg.StageId}).Debug("Completing delayed stage")
		if !g.validateCmd(msg, context) {
			return
		}
		completedEvent := &model.StageCompletedEvent{
			StageId: msg.StageId,
			Result:  msg.Result,
		}
		g.persist(completedEvent)
		g.applyStageCompletedEvent(completedEvent)

	case *model.FaasInvocationResponse:
		g.log.WithFields(logrus.Fields{"stage_id": msg.StageId}).Debug("Received fn invocation response")

	default:
		g.log.Infof("Ignoring message of unknown type %v", reflect.TypeOf(msg))
	}
}

func (g *graphActor) Receive(context actor.Context) {
	if g.Recovering() {
		g.receiveRecover(context)
	} else {
		g.receiveStandard(context)
	}
}

func (g *graphActor) OnExecuteStage(stage *graph.CompletionStage, datum []*model.Datum) {
	g.log.WithField("stage_id", stage.ID).Info("Executing Stage")

	msg := &model.InvokeStageRequest{FunctionId: g.graph.FunctionID, GraphId: g.graph.ID, StageId: stage.ID, Args: datum, Closure: stage.GetClosure(), Operation: stage.GetOperation()}

	g.executor.Request(msg, g.GetSelf())
}

//OnCompleteStage indicates that a stage is finished and its result is available
func (g *graphActor) OnCompleteStage(stage *graph.CompletionStage, result *model.CompletionResult) {
	g.log.WithField("stage_id", stage.ID).Info("Completing stage in OnCompleteStage")
	completedEvent := &model.StageCompletedEvent{
		StageId: stage.ID,
		Result:  result,
	}
	g.persist(completedEvent)
	g.applyStageCompletedEvent(completedEvent)
}

//OnCompose Stage indicates that another stage should be composed into this one
func (g *graphActor) OnComposeStage(stage *graph.CompletionStage, composedStage *graph.CompletionStage) {
	g.log.WithField("stage_id", stage.ID).Info("Composing stage in OnComposeStage")
	composedEvent := &model.StageComposedEvent{
		StageId:         stage.ID,
		ComposedStageId: composedStage.ID,
	}
	g.persist(composedEvent)
	g.applyStageComposedEvent(composedEvent)
}

//OnCompleteGraph indicates that the graph is now finished and cannot be modified
func (g *graphActor) OnCompleteGraph() {
	g.log.Info("Completing graph in OnCompleteGraph")
	completedEvent := &model.GraphCompletedEvent{
		GraphId:    g.graph.ID,
		FunctionId: g.graph.FunctionID,
	}
	g.persist(completedEvent)
	g.applyGraphCompletedEvent(completedEvent)
}
