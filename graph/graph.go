package graph

import (
	"github.com/sirupsen/logrus"
	"github.com/fnproject/completer/model"
)

type GraphId string

type StageId uint32

type CompletionStage struct {
	Id           StageId
	operation    model.CompletionOperation
	closure      *model.RawDatum
	result       *model.CompletionResult
	dependencies []StageId
	children     []StageId
	// TODO when complete future

	// Reference because it is nullable
	composeReference *StageId
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

func New (id GraphId, functionId string, listener CompletionEventListener) *CompletionGraph {
	return &CompletionGraph {
		Id:             id,
		FunctionId:     functionId,
		stages:         make(map[StageId]*CompletionStage),
		eventProcessor: listener,
		log:            logrus.WithFields(logrus.Fields{"graph_id": id, "function_id": functionId, "logger": "graph"}),
		committed:      false,
		completed:      false,
	}
}

func (graph *CompletionGraph) SetCommitted() {
	graph.log.Info("committing graph")
	graph.committed = true
	graph.checkForCompletions()
}

func (graph *CompletionGraph) IsCommitted () bool {
	return graph.committed
}

func (graph *CompletionGraph) SetCompleted() {
	graph.log.Info("completing graph")
	graph.completed = true
}

func (graph *CompletionGraph) IsCompleted () bool {
	return graph.completed
}

func (graph *CompletionGraph) GetStage(stageId StageId) *CompletionStage {
	return graph.stages[stageId]
}

func toStageIdArray(in []uint32) []StageId {
	res := make([]StageId, len(in))
	for i := 0; i < len(in); i++ {
		res[i] = StageId(in[i])
	}
	return res
}

func (graph *CompletionGraph) AddStage(event *model.StageAddedEvent, shouldTrigger bool) {
	node := &CompletionStage {
		Id: StageId(event.StageId),
		operation: event.Op,
		closure: event.Closure,
		result: nil,
		dependencies: toStageIdArray(event.Dependencies),
	}
	for i := 0; i < len(node.dependencies) ; i++ {
		graph.stages[node.dependencies[i]].children = append(graph.stages[node.dependencies[i]].children, node.Id)
	}

	// TODO: more stuff

	graph.stages[node.Id] = node
}


func (graph *CompletionGraph) checkForCompletions() {}