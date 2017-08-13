package graph

import (
	"github.com/fnproject/completer/model"
	"github.com/sirupsen/logrus"
	"fmt"
)

var log *logrus.Entry = logrus.WithField("logger", "graph")

type GraphId string

type StageId uint32

type TriggerStatus bool

const (
	TriggerStatus_successful TriggerStatus = true
	TriggerStatus_failed     TriggerStatus = false
)

type CompletionEventListener interface {
	OnExecuteStage(stage *CompletionStage, datum []*model.Datum)
	OnCompleteStage(stage *CompletionStage, result *model.CompletionResult)
	OnComposeStage(stage *CompletionStage, composedStage *CompletionStage)
	OnCompleteGraph()
}

// Completion Graph

type CompletionGraph struct {
	Id         GraphId
	FunctionId string

	stages map[StageId]*CompletionStage

	eventProcessor CompletionEventListener

	log *logrus.Entry

	committed bool
	completed bool
}

func New(id GraphId, functionId string, listener CompletionEventListener) *CompletionGraph {
	return &CompletionGraph{
		Id:             id,
		FunctionId:     functionId,
		stages:         make(map[StageId]*CompletionStage),
		eventProcessor: listener,
		log:            log.WithFields(logrus.Fields{"graph_id": id, "function_id": functionId}),
		committed:      false,
		completed:      false,
	}
}

func (graph *CompletionGraph) IsCommitted() bool {
	return graph.committed
}

func (graph *CompletionGraph) SetCommitted() {
	graph.log.Info("committing graph")
	graph.committed = true
	graph.checkForCompletion()
}

func (graph *CompletionGraph) IsCompleted() bool {
	return graph.completed
}

func (graph *CompletionGraph) SetCompleted() {
	graph.log.Info("completing graph")
	graph.completed = true
}

func (graph *CompletionGraph) GetStage(stageId StageId) *CompletionStage {
	return graph.stages[stageId]
}

func (graph *CompletionGraph) GetStages(stageIds []StageId) []*CompletionStage {
	res := make([]*CompletionStage, len(stageIds))
	for i, id := range stageIds {
		res[i] = graph.GetStage(id)
	}
	return res
}

func (graph *CompletionGraph) GetAllStages() []*CompletionStage {
	res := make([]*CompletionStage, len(graph.stages))
	for i, s := range graph.stages {
		res[i] = s
	}
	return res
}

func (graph *CompletionGraph) NextStageId() uint32 {
	return uint32(len(graph.stages))
}

func toStageIdArray(in []uint32) []StageId {
	res := make([]StageId, len(in))
	for i, s := range in {
		res[i] = StageId(s)
	}
	return res
}

func (graph *CompletionGraph) AddStage(event *model.StageAddedEvent, shouldTrigger bool) error {
	log := graph.log.WithFields(logrus.Fields{"stage": event.StageId, "op": event.Op})

	strategy, err := GetStrategyFromOperation(event.Op)

	if err != nil {
		log.Errorf("invalid/unsupported operation %s", event.Op)
		return err
	}

	if graph.IsCompleted(){
		log.Errorf("Graph already completed")
		return fmt.Errorf("Graph already completed")
	}
	_, hasStage := graph.stages[StageId(event.StageId)]

	if hasStage {
		log.Error("Duplicate stage")
		return fmt.Errorf("Duplicate stage %d",event.StageId)
	}
	log.Info("Adding stage to graph")
	node := &CompletionStage{
		Id:           StageId(event.StageId),
		operation:    event.Op,
		strategy:     strategy,
		closure:      event.Closure,
		whenComplete: make(chan bool),
		result:       nil,
		dependencies: graph.GetStages(toStageIdArray(event.Dependencies)),
	}
	for _, dep := range node.dependencies {
		dep.children = append(dep.children, node)
	}
	graph.stages[node.Id] = node

	if shouldTrigger {
		graph.executeCompletableStages()
	}
	return nil
}

// External actor triggers state as completed , this may trigger further stages
func (graph *CompletionGraph) CompleteStage(event *model.StageCompletedEvent, shouldTrigger bool) bool {
	log := graph.log.WithFields(logrus.Fields{"status": event.Result.Status, "stage_id": event.StageId})
	log.Info("Completing node")
	node := graph.stages[StageId(event.StageId)]
	success := node.complete(event.Result)

	if node.composeReference != nil {
		graph.tryCompleteComposedStage(node.composeReference, node)
	}

	if shouldTrigger {
		graph.executeCompletableStages()
	}

	graph.checkForCompletion()

	return success
}

// InvokeComplete is signaled when an invocation (or function call) associated with a stage is completed
// This may signal completion of the stage (in which case a Complete Event is raised
func (graph *CompletionGraph) InvokeComplete(stageId StageId, result *model.CompletionResult) {
	log.WithField("stage_id", stageId).Info("Completing stage with faas response");
	stage := graph.stages[stageId]
	strategy := stage.strategy
	strategy.ResultHandlingStrategy(stage, graph, result)
}

func (graph *CompletionGraph) ComposeNodes(event *model.StageComposedEvent) {
	log := graph.log.WithFields(logrus.Fields{"stage": event.StageId, "composed_stage": event.ComposedStageId})
	log.Info("Setting composed reference")
	outer := graph.stages[StageId(event.StageId)]
	inner := graph.stages[StageId(event.ComposedStageId)]
	inner.composeReference = outer

	// If inner stage is complete, complete outer stage too
	graph.tryCompleteComposedStage(outer, inner)
}

func (graph *CompletionGraph) Recover() {
	for _, stage := range graph.stages {
		if !stage.isResolved() {
			triggerStatus, _, _ := stage.requestTrigger()
			if triggerStatus {
				graph.log.WithFields(logrus.Fields{"stage": stage.Id}).Info("Failing irrecoverable node")
				graph.eventProcessor.OnCompleteStage(stage, stageRecoveryFailedResult)
			}
		}
	}

	if len(graph.stages) > 0 {
		graph.log.Info("Retrieved stages from storage")
	}

	graph.checkForCompletion()
}

var stageRecoveryFailedResult *model.CompletionResult = internalErrorResult(model.ErrorDatumType_stage_lost, "Stage invocation lost - stage may still be executing")

func (graph *CompletionGraph) tryCompleteComposedStage(outer *CompletionStage, inner *CompletionStage) {
	if inner.isResolved() && !outer.isResolved() {
		graph.log.WithFields(logrus.Fields{"stage": outer.Id, "inner_stage": inner.Id}).Info("Completing composed stage with inner stage")
		graph.eventProcessor.OnCompleteStage(outer, inner.result)
	}
}

func (graph *CompletionGraph) executeCompletableStages() {
	for _, stage := range graph.stages {
		if !stage.isResolved() {
			triggered, status, input := stage.requestTrigger()
			if triggered {
				log := graph.log.WithFields(logrus.Fields{"stage_id": stage.Id, "status": status})
				log.Info("Preparing to execute node")
				var strategy ExecutionStrategy
				if status == TriggerStatus_failed {
					strategy = stage.strategy.FailureStrategy
				} else {
					strategy = stage.strategy.SuccessStrategy
				}
				strategy(stage, graph.eventProcessor, input)
			}
		}
	}
}

func (graph *CompletionGraph) getPendingCount() uint32 {
	count := uint32(0)
	for _, s := range graph.stages {
		if !s.isResolved() {
			count++
		}
	}
	return count
}

func (graph *CompletionGraph) checkForCompletion() {
	pendingCount := graph.getPendingCount()
	if graph.IsCommitted() && !graph.IsCompleted() && pendingCount == 0 {
		graph.log.Info("Graphsuccessfully completed")
		graph.eventProcessor.OnCompleteGraph()
	} else {
		graph.log.WithFields(logrus.Fields{"pending": pendingCount}).Info("Pending executions before graph can be completed")
	}
}
