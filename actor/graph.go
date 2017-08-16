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
	"context"
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

// TODO: Reified completion event listener
type completionEventListenerImpl struct {
	actor *graphActor
}

func (listener *completionEventListenerImpl) OnExecuteStage(stage *graph.CompletionStage, datum []*model.Datum) {
}
func (listener *completionEventListenerImpl) OnCompleteStage(stage *graph.CompletionStage, result *model.CompletionResult) {
}
func (listener *completionEventListenerImpl) OnComposeStage(stage *graph.CompletionStage, composedStage *graph.CompletionStage) {
}
func (listener *completionEventListenerImpl) OnCompleteGraph() {}

func (g *graphActor) persist(event proto.Message) error {
	g.PersistReceive(event)
	return nil
}

func (g *graphActor) applyGraphCreatedEvent(event *model.GraphCreatedEvent) {
	g.log.WithFields(logrus.Fields{"graph_id": event.GraphId, "function_id": event.FunctionId}).Debug("Creating completion graph")
	listener := &completionEventListenerImpl{actor: g}
	g.graph = graph.New(event.GraphId, event.FunctionId, listener)
}

func (g *graphActor) applyGraphCommittedEvent(event *model.GraphCommittedEvent) {
	g.log.WithFields(logrus.Fields{"graph_id": g.graph.ID, "function_id": g.graph.FunctionID}).Debug("Committing graph")
	g.graph.HandleCommitted()
}

func (g *graphActor) applyGraphCompletedEvent(event *model.GraphCompletedEvent) {
	g.log.WithFields(logrus.Fields{"graph_id": g.graph.ID, "function_id": g.graph.FunctionID}).Debug("Completing graph")
	g.graph.HandleCompleted()
	// "poison pill"
	g.pid.Stop()
}

func (g *graphActor) applyStageAddedEvent(event *model.StageAddedEvent) {
	g.log.WithFields(logrus.Fields{"graph_id": g.graph.ID, "function_id": g.graph.FunctionID, "stage_id": event.StageId}).Debug("Adding stage")
	g.graph.HandleStageAdded(event, !g.Recovering())
}

func (g *graphActor) applyStageCompletedEvent(event *model.StageCompletedEvent) {
	g.log.WithFields(logrus.Fields{"graph_id": g.graph.ID, "function_id": g.graph.FunctionID, "stage_id": event.StageId}).Debug("Completing stage")
	g.graph.HandleStageCompleted(event, !g.Recovering())
}

func (g *graphActor) applyStageComposedEvent(event *model.StageComposedEvent) {
	g.log.WithFields(logrus.Fields{"graph_id": g.graph.ID, "function_id": g.graph.FunctionID, "stage_id": event.StageId}).Debug("Composing stage")
	g.graph.HandleStageComposed(event)
}

func (g *graphActor) applyDelayScheduledEvent(event *model.DelayScheduledEvent) {
	// we always need to complete delay nodes from scratch to avoid completing twice
	delayMs := int64(event.DelayedTs) - timeMillis()
	if delayMs > 0 {
		g.log.WithFields(logrus.Fields{"graph_id": g.graph.ID, "function_id": g.graph.FunctionID, "stage_id": event.StageId}).Debug("Scheduling delayed completion of stage")
		// Wait for the delay in a goroutine so we can complete the request in the meantime
		go func() {
			timer := make(chan bool, 1)
			go func() {
				time.Sleep(time.Duration(delayMs) * time.Millisecond)
				timer <- true
			}()
			if <-timer {
				g.pid.Tell(model.CompleteDelayStageRequest{
					string(g.graph.ID),
					event.StageId,
					model.NewSuccessfulResult(&model.Datum{Val: &model.Datum_Empty{Empty: &model.EmptyDatum{}}})})
			}
		}()
	} else {
		g.log.WithFields(logrus.Fields{"graph_id": g.graph.ID, "function_id": g.graph.FunctionID, "stage_id": event.StageId}).Debug("Queuing completion of delayed stage")
		g.pid.Tell(model.CompleteDelayStageRequest{
			string(g.graph.ID),
			event.StageId,
			model.NewSuccessfulResult(&model.Datum{Val: &model.Datum_Empty{Empty: &model.EmptyDatum{}}})})
	}
}

func timeMillis() int64 {
	return time.Now().UnixNano() / 1000000
}

func (g *graphActor) applyNoop(event interface{}) {

}

// process events
func (g *graphActor) receiveRecover(context actor.Context) {
}

func (g *graphActor) validateStages(stageIDs []string) bool {
	return g.graph.GetStages(stageIDs) != nil
}

// if validation fails, this method will respond to the request with an appropriate error message
func (g *graphActor) validateCmd(cmd interface{}, context actor.Context) bool {
	if isGraphMessage(cmd) {
		graphId := getGraphId(cmd)
		if g.graph == nil {
			context.Respond(NewGraphNotFoundError(graphId))
			return false
		} else if g.graph.IsCompleted() {
			context.Respond(NewGraphCompletedError(graphId))
			return false
		}
	}

	switch msg := cmd.(type) {

	case *model.AddDelayStageRequest:

	case *model.AddChainedStageRequest:
		if g.validateStages(msg.Deps) {
			context.Respond(NewGraphCompletedError(msg.GraphId))
			return false
		}
	}

	return true
}

// process commands
func (g *graphActor) receiveStandard(context actor.Context) {
	switch msg := context.Message().(type) {

	case *model.CreateGraphRequest:
		g.log.WithFields(logrus.Fields{"graph_id": msg.GraphId}).Debug("Creating graph")
		event := &model.GraphCreatedEvent{GraphId: msg.GraphId, FunctionId: msg.FunctionId}
		err := g.persist(event)
		if err != nil {
			context.Respond(NewGraphEventPersistenceError(msg.GraphId))
			return
		}
		g.applyGraphCreatedEvent(event)
		context.Respond(&model.CreateGraphResponse{GraphId: msg.GraphId})

	case *model.AddChainedStageRequest:
		if !g.validateCmd(msg, context) {
			return
		}
		g.log.WithFields(logrus.Fields{"graph_id": msg.GraphId}).Debug("Adding chained stage")
		event := &model.StageAddedEvent{
			StageId:      g.graph.NextStageID(),
			Op:           msg.Operation,
			Closure:      msg.Closure,
			Dependencies: msg.Deps,
		}
		err := g.persist(event)
		if err != nil {
			context.Respond(NewGraphEventPersistenceError(msg.GraphId))
			return
		}
		g.applyNoop(event)
		context.Respond(&model.AddStageResponse{GraphId: msg.GraphId, StageId: event.StageId})

	case *model.AddCompletedValueStageRequest:
		if !g.validateCmd(msg, context) {
			return
		}
		g.log.WithFields(logrus.Fields{"graph_id": msg.GraphId}).Debug("Adding completed value stage")

		addedEvent := &model.StageAddedEvent{
			StageId: g.graph.NextStageID(),
			Op:      model.CompletionOperation_completedValue,
		}
		err := g.persist(addedEvent)
		if err != nil {
			context.Respond(NewGraphEventPersistenceError(msg.GraphId))
			return
		}
		g.applyNoop(addedEvent)

		completedEvent := &model.StageCompletedEvent{
			StageId: g.graph.NextStageID(),
			Result:  msg.Result,
		}
		err = g.persist(completedEvent)
		if err != nil {
			context.Respond(NewGraphEventPersistenceError(msg.GraphId))
			return
		}
		g.applyNoop(completedEvent)
		context.Respond(&model.AddStageResponse{GraphId: msg.GraphId, StageId: addedEvent.StageId})

	case *model.AddDelayStageRequest:
		if !g.validateCmd(msg, context) {
			return
		}
		g.log.WithFields(logrus.Fields{"graph_id": msg.GraphId}).Debug("Adding delay stage")

		addedEvent := &model.StageAddedEvent{
			StageId: g.graph.NextStageID(),
			Op:      model.CompletionOperation_delay,
		}
		err := g.persist(addedEvent)
		if err != nil {
			context.Respond(NewGraphEventPersistenceError(msg.GraphId))
			return
		}
		g.applyNoop(addedEvent)

		delayEvent := &model.DelayScheduledEvent{
			StageId:   g.graph.NextStageID(),
			DelayedTs: uint64(timeMillis()) + msg.DelayMs,
		}
		err = g.persist(delayEvent)
		if err != nil {
			context.Respond(NewGraphEventPersistenceError(msg.GraphId))
			return
		}
		g.applyNoop(delayEvent)

		context.Respond(&model.AddStageResponse{GraphId: msg.GraphId, StageId: addedEvent.StageId})

	case *model.AddExternalCompletionStageRequest:
		if !g.validateCmd(msg, context) {
			return
		}
		g.log.WithFields(logrus.Fields{"graph_id": msg.GraphId}).Debug("Adding external completion stage")
		event := &model.StageAddedEvent{
			StageId: g.graph.NextStageID(),
			Op:      model.CompletionOperation_externalCompletion,
		}
		err := g.persist(event)
		if err != nil {
			context.Respond(NewGraphEventPersistenceError(msg.GraphId))
			return
		}
		g.applyNoop(event)
		context.Respond(&model.AddStageResponse{GraphId: msg.GraphId, StageId: event.StageId})

	case *model.AddInvokeFunctionStageRequest:
		if !g.validateCmd(msg, context) {
			return
		}
		g.log.WithFields(logrus.Fields{"graph_id": msg.GraphId}).Debug("Adding invoke stage")

		event := &model.StageAddedEvent{
			StageId: g.graph.NextStageID(),
			Op:      model.CompletionOperation_completedValue,
		}
		err := g.persist(event)
		if err != nil {
			context.Respond(NewGraphEventPersistenceError(msg.GraphId))
			return
		}
		g.applyNoop(event)

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
		g.log.WithFields(logrus.Fields{"graph_id": msg.GraphId}).Debug("Completing stage externally")
		context.Respond(&model.CompleteStageExternallyResponse{GraphId: msg.GraphId, StageId: msg.StageId, Successful: true})

	case *model.CommitGraphRequest:
		g.log.WithFields(logrus.Fields{"graph_id": msg.GraphId}).Debug("Committing graph")
		context.Respond(&model.CommitGraphProcessed{GraphId: msg.GraphId})

	case *model.GetStageResultRequest:
		g.log.WithFields(logrus.Fields{"graph_id": msg.GraphId}).Debug("Retrieving stage result")
		datum := &model.Datum{
			Val: &model.Datum_Blob{
				Blob: &model.BlobDatum{ContentType: "text", DataString: []byte("foo")},
			},
		}
		result := &model.CompletionResult{Successful: true, Datum: datum}
		context.Respond(&model.GetStageResultResponse{GraphId: msg.GraphId, StageId: msg.StageId, Result: result})

	case *model.CompleteDelayStageRequest:
		g.log.WithFields(logrus.Fields{"graph_id": msg.GraphId}).Debug("Completing delayed stage")

	case *model.FaasInvocationResponse:
		g.log.WithFields(logrus.Fields{"graph_id": msg.GraphId}).Debug("Received fn invocation response")

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
