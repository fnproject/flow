package graph

import (
	"github.com/fnproject/completer/actor/messages"
	"github.com/fnproject/completer/model"
	"github.com/sirupsen/logrus"
	"errors"
)

type GraphId string

type StageId uint32

type TriggerStatus uint8
const (
	TriggerStatus_successful  TriggerStatus = 0
	TriggerStatus_exceptional TriggerStatus = 1
	TriggerStatus_pending     TriggerStatus = 2
)
func (ts TriggerStatus) isCompletable() bool { return ts == TriggerStatus_pending }
func (ts TriggerStatus) isExceptional() bool { return ts == TriggerStatus_exceptional }

type CompletionEventListener interface {
	OnCompleteStage(stage *CompletionStage, result *model.CompletionResult)
	OnCompleteGraph()
}

// Completion Stage

type CompletionStage struct {
	Id           StageId
	operation    model.CompletionOperation
	closure      *model.RawDatum
	result       *model.CompletionResult
	dependencies []StageId
	children     []StageId

	// TODO "when complete" future

	// Reference because it is nullable
	composeReference *StageId
}

func (stage *CompletionStage) Complete(result *model.CompletionResult) bool {
	// TODO
	return false
}

func (stage *CompletionStage) isResolved() bool {
	return stage.result != nil
}

func (stage *CompletionStage) isCompleted() bool {
	return stage.isResolved() && stage.result.Status == model.ResultStatus_succeeded
}

func (stage *CompletionStage) isFailed() bool {
	return stage.isResolved() && (stage.result.Status == model.ResultStatus_failed || stage.result.Status == model.ResultStatus_error)
}

// Some static functions useful for Completion Stage

func isOperationSatisfied(operation model.CompletionOperation, dependencies []*CompletionStage) bool {
	switch GetStrategyFromOperation(operation).Trigger {
	case triggerImmediate:
		return true
	case triggerNever:
		return false
	case triggerAny:
		// "if any one is resolved"
		for _,s := range dependencies {
			if s.isResolved() {
				return true
			}
		}
		return false
	case triggerAll:
		// "all must be resolved"
		for _,s := range dependencies {
			if !s.isResolved() {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func getTriggerStatus(operation model.CompletionOperation, dependencies []*CompletionStage) TriggerStatus {
	for _,d := range dependencies {
		if d.isFailed() {
			return TriggerStatus_exceptional
		}
	}
	if isOperationSatisfied(operation, dependencies) {
		return TriggerStatus_successful
	}
	return TriggerStatus_pending
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
		log:            logrus.WithFields(logrus.Fields{"graph_id": id, "function_id": functionId, "logger": "graph"}),
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
	if stageId >= StageId(len(graph.stages)) {
		return nil
	}
	return graph.stages[stageId]
}

func (graph *CompletionGraph) GetStages(stageIds []StageId) []*CompletionStage {
	res := make([]*CompletionStage, len(stageIds))
	for i,id := range stageIds {
		res[i] = graph.GetStage(id)
	}
	return res
}

func (graph *CompletionGraph) GetAllStages() []*CompletionStage {
	res := make([]*CompletionStage, len(graph.stages))
	for i,s := range graph.stages {
		res[i] = s
	}
	return res
}

func (graph *CompletionGraph) NextStageId() uint32 {
	return uint32(len(graph.stages))
}

func toStageIdArray(in []uint32) []StageId {
	res := make([]StageId, len(in))
	for i,s := range in {
		res[i] = StageId(s)
	}
	return res
}

func (graph *CompletionGraph) AddStage(event *model.StageAddedEvent, shouldTrigger bool) {
	log := graph.log.WithFields(logrus.Fields{"stage": event.StageId, "op": event.Op})
	log.Info("Adding stage to graph")
	if event.StageId < uint32(len(graph.stages)) {
		log.Warn("New stage will overwrite previous stage")
	}
	node := &CompletionStage{
		Id:           StageId(event.StageId),
		operation:    event.Op,
		closure:      event.Closure,
		result:       nil,
		dependencies: toStageIdArray(event.Dependencies),
	}
	for i := 0; i < len(node.dependencies); i++ {
		graph.stages[node.dependencies[i]].children = append(graph.stages[node.dependencies[i]].children, node.Id)
	}
	graph.stages[node.Id] = node

	if shouldTrigger {
		graph.executeCompletableStages([]StageId{node.Id})
	}
}

func (graph *CompletionGraph) CompleteStage(event *model.StageCompletedEvent, shouldTrigger bool) bool {
	log := graph.log.WithFields(logrus.Fields{"status": event.Result.Status})
	log.Info("Completing node")
	node := graph.stages[StageId(event.StageId)]
	success := node.Complete(event.Result)

	if node.composeReference != nil {
		graph.tryCompleteComposedStage(graph.stages[node.composeReference], node)
	}

	if shouldTrigger {
		graph.executeCompletableStages(node.children)
	}

	graph.checkForCompletion()

	return success
}

func (graph *CompletionGraph) ComposeNodes(event model.StageComposedEvent) {
	log := graph.log.WithFields(logrus.Fields{"stage": event.StageId, "composed_stage": event.ComposedStageId})
	log.Info("Setting composed reference")
	outer := graph.stages[StageId(event.StageId)]
	inner := graph.stages[StageId(event.ComposedStageId)]
	inner.composeReference = &outer.Id

	// If inner stage is complete, complete outer stage too
	graph.tryCompleteComposedStage(outer, inner)
}

func (graph *CompletionGraph) ExecuteStage(stage *CompletionStage, status TriggerStatus) {
	log := graph.log.WithFields(logrus.Fields{"stage": stage.Id})
	log.Info("Preparing to execute node")
	var e error
	if status.isExceptional() {
		e = graph.executeExceptionally(stage)
	} else {
		e = graph.executeNormally(stage)
	}
	if e != nil {
		log.WithFields(logrus.Fields{"error": e}).Info("Failed to execute stage")
		graph.eventProcessor.OnCompleteStage(stage, messages.FailedFromPlatformError(e))
	}
}

func (graph *CompletionGraph) CompleteWithInvokeResult(stageId uint32, result *model.CompletionResult) {
	log := graph.log.WithFields(logrus.Fields{"stage": stageId})
	log.Info("Completing stage with FaaS response")
	stage := graph.stages[StageId(stageId)]
	var e error = nil
	switch GetStrategyFromOperation(stage.operation).ResultHandlingStrategy {
	case referencedStageResult:
		e = graph.handleStageReferenceCompletion(stage, result)
		break
	case invocationResult:
		graph.eventProcessor.OnCompleteStage(stage, result)
		break
	case parentStageResult:
		depResult := graph.getDepResult(stage)
		if depResult != nil {
			graph.eventProcessor.OnCompleteStage(stage, depResult)
		}
		break
	default:
		errorMessage := "Invalid node result strategy"
		log.WithFields(logrus.Fields{"result_strategy": strategy}).Warn(errorMessage)
		e = errors.New(errorMessage)
	}
	if e != nil {
		graph.eventProcessor.OnCompleteStage(stage, messages.FailedFromPlatformError(e))
	}
}

func (graph *CompletionGraph) Recover() {
	graph.visitAllCompletable(func(stage *CompletionStage, status TriggerStatus) {
		graph.log.WithFields(logrus.Fields{"stage": stage.Id}).Info("Failing irrecoverable node")
		graph.eventProcessor.OnCompleteStage(stage, messages.FailedFromPlatformError(errors.New("Interrupted stage")))
	})

	if len(graph.stages) > 0 {
		graph.log.Info("Retrieved stages from storage")
	}

	graph.checkForCompletion()
}

func (graph *CompletionGraph) executeExceptionally(target *CompletionStage) error {
	// TODO
}

func (graph *CompletionGraph) executeNormally(target *CompletionStage) error {
	// TODO
}

func (graph *CompletionGraph) handleStageReferenceCompletion(stage *CompletionStage, result *model.CompletionResult) error {
	// TODO
	// RELIES ON JAVA SERIALIZATION
}

func (graph *CompletionGraph) tryCompleteComposedStage(outer *CompletionStage, inner *CompletionStage) {
	if inner.isResolved() && !outer.isResolved() {
		graph.log.WithFields(logrus.Fields{"outer_stage": outer.Id, "inner_stage": inner.Id}).Info("Completing composed stage with inner stage")
		graph.eventProcessor.OnCompleteStage(outer, inner.result)
	}
}

func (graph *CompletionGraph) executeCompletableStages(id []StageId) {
	graph.visitAllCompletable(func(stage *CompletionStage, status TriggerStatus) {
		graph.ExecuteStage(stage, status)
	})
}

func (graph *CompletionGraph) getPendingCount() uint32 {
	count := uint32(0)
	for _,s := range graph.stages {
		if s.isResolved() {
			count++
		}
	}
	return count
}

func (graph *CompletionGraph) checkForCompletion() {
	pendingCount := graph.getPendingCount()
	if graph.IsCommitted() && !graph.IsCompleted() && pendingCount == 0 {
		graph.log.Info("Graph successfully completed")
		graph.eventProcessor.OnCompleteGraph()
	} else {
		graph.log.WithFields(logrus.Fields{"pending": pendingCount}).Info("Pending executions before graph can be completed")
	}
}

// Visitor methods

func (graph *CompletionGraph) getDepResult(stage *CompletionStage) *model.CompletionResult {
	if len(stage.dependencies) == 0 {
		return nil
	}
	// TODO: this method is useful only if there is one dependency. What about other cases?
	return graph.GetStage(stage.dependencies[0]).result
}

func (graph *CompletionGraph) visitAllCompletable(f func(stage *CompletionStage, status TriggerStatus)) {
	graph.visitCompletable(graph.GetAllStages(), f)
}

func (graph *CompletionGraph) visitCompletableById(stageIds []StageId, f func(stage *CompletionStage, status TriggerStatus)) {
	graph.visitCompletable(graph.GetStages(stageIds), f)
}

func (graph *CompletionGraph) visitCompletable(stages []*CompletionStage, f func(stage *CompletionStage, status TriggerStatus)) {
	for _,s := range stages {
		f(s, getTriggerStatus(s.operation, graph.GetStages(s.dependencies)))
	}
}