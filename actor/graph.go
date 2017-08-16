package actor

import (
	"github.com/fnproject/completer/model"
	"github.com/sirupsen/logrus"
	"github.com/fnproject/completer/graph"
	"github.com/AsynkronIT/protoactor-go/actor"
)

type graphActor struct {
	PIDHolder
	graph    *graph.CompletionGraph
	log      *logrus.Entry
	executor *actor.PID
}

func NewGraphActor(graphId string, functionId string, executor *actor.PID) *graphActor {
	graphActor := &graphActor{
		executor: executor,
		log:      logrus.WithFields(logrus.Fields{"logger": "graph_actor", "graph_id": graphId, "function_id": functionId}),
	}

	graphModel := graph.New(graphId, functionId, graphActor)
	graphActor.graph = graphModel
	return graphActor
}

func (g *graphActor) Receive(context actor.Context) {
	switch msg := context.Message().(type) {

	case *model.AddChainedStageRequest:
		g.log.Debug("Adding chained stage")
		context.Respond(&model.AddStageResponse{GraphId: msg.GraphId, StageId: "1"})

	case *model.AddCompletedValueStageRequest:
		g.log.Debug("Adding completed value stage")
		context.Respond(&model.AddStageResponse{GraphId: msg.GraphId, StageId: "1"})

	case *model.AddDelayStageRequest:
		g.log.Debug("Adding delay stage")
		context.Respond(&model.AddStageResponse{GraphId: msg.GraphId, StageId: "1"})

	case *model.AddExternalCompletionStageRequest:
		g.log.Debug("Adding external completion stage")
		context.Respond(&model.AddStageResponse{GraphId: msg.GraphId, StageId: "1"})

	case *model.AddInvokeFunctionStageRequest:
		log.WithFields(logrus.Fields{"graph_id": msg.GraphId}).Debug("Adding invoke stage")
		context.Respond(&model.AddStageResponse{GraphId: msg.GraphId, StageId: "1"})

	case *model.CompleteStageExternallyRequest:
		g.log.Debug("Completing stage externally")
		context.Respond(&model.CompleteStageExternallyResponse{GraphId: msg.GraphId, StageId: msg.StageId, Successful: true})

	case *model.CommitGraphRequest:
		g.log.Debug("Committing graph")
		context.Respond(&model.CommitGraphProcessed{GraphId: msg.GraphId})

	case *model.GetStageResultRequest:
		g.log.Debug("Retrieving stage result")
		datum := &model.Datum{
			Val: &model.Datum_Blob{
				Blob: &model.BlobDatum{ContentType: "text", DataString: []byte("foo")},
			},
		}
		result := &model.CompletionResult{Successful: true, Datum: datum}
		context.Respond(&model.GetStageResultResponse{GraphId: msg.GraphId, StageId: msg.StageId, Result: result})

	case *model.CompleteDelayStageRequest:
		log.WithFields(logrus.Fields{"graph_id": msg.GraphId}).Debug("Completing delayed stage")

	case *model.FaasInvocationResponse:
		log.WithFields(logrus.Fields{"graph_id": msg.GraphId}).Debug("Received fn invocation response")
		g.graph.HandleInvokeComplete(msg.StageId,msg.Result)

	}

}

func (g *graphActor) OnExecuteStage(stage *graph.CompletionStage, datum []*model.Datum) {
	g.log.WithField("stage_id", stage.ID).Info("Executing Stage")

	msg := &model.InvokeStageRequest{FunctionId: g.graph.FunctionID, GraphId: g.graph.ID, StageId: stage.ID, Args: datum, Closure: stage.GetClosure(), Operation: stage.GetOperation()}

	g.executor.Request(msg, g.GetSelf())
}

//OnCompleteStage indicates that a stage is finished and its result is available
func (g *graphActor) OnCompleteStage(stage *graph.CompletionStage, result *model.CompletionResult) {
	g.graph.HandleStageCompleted(&model.StageCompletedEvent{StageId: stage.ID, Result: result}, true)
}

//OnCompose Stage indicates that another stage should be composed into this one
func (g *graphActor) OnComposeStage(stage *graph.CompletionStage, composedStage *graph.CompletionStage) {
	g.graph.HandleStageComposed(&model.StageComposedEvent{StageId: stage.ID, ComposedStageId: composedStage.ID})

}

//OnCompleteGraph indicates that the graph is now finished and cannot be modified
func (*graphActor) OnCompleteGraph() {

}
