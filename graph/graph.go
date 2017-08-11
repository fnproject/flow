package graph

import (
	"github.com/fnproject/completer/model"
	"github.com/sirupsen/logrus"
)

type GraphId string

type StageId uint32

type TriggerStatus uint32
const (
	SUCCESSFUL  TriggerStatus = 0
	EXCEPTIONAL TriggerStatus = 1
	PENDING     TriggerStatus = 2
)
func (ts TriggerStatus) isCompletable() bool { return ts == PENDING }
func (ts TriggerStatus) isExceptional() bool { return ts == EXCEPTIONAL }

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

type CompletionEventListener interface {
}

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

// TODO: Do we need this?!?
func (graph *CompletionGraph) GetStages() *map[StageId]*CompletionStage {
	return &graph.stages
}

func (graph *CompletionGraph) GetTriggerStatus(stage CompletionStage) *TriggerStatus {
	// TODO
	return nil
}

func (graph *CompletionGraph) NextStageId() uint32 {
	return uint32(len(graph.stages))
}

func toStageIdArray(in []uint32) []StageId {
	res := make([]StageId, len(in))
	for i := 0; i < len(in); i++ {
		res[i] = StageId(in[i])
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
		graph.tryCompleteComposedStage(graph[node.composeReference], node)
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

func (graph *CompletionGraph) tryCompleteComposedStage(outer *CompletionStage, inner *CompletionStage) {
	// TODO
}

func (graph *CompletionGraph) executeCompletableStages(id []StageId) {
	// TODO
}

func (graph *CompletionGraph) checkForCompletion() {
	// TODO
}
