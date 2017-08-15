// Package graph describes a persistent, reliable completion graph for a process
package graph

import (
	"fmt"
	"github.com/fnproject/completer/model"
	"github.com/sirupsen/logrus"
	"strconv"
)

var log = logrus.WithField("logger", "graph")


// CompletionEventListener is a callback interface to receive notifications about stage triggers and graph events
type CompletionEventListener interface {
	//OnExecuteStage indicates that  a stage is due to be executed with the given arguments
	OnExecuteStage(stage *CompletionStage, datum []*model.Datum)
	//OnCompleteStage indicates that a stage is finished and its result is available
	OnCompleteStage(stage *CompletionStage, result *model.CompletionResult)
	//OnCompose Stage indicates that another stage should be composed into this one
	OnComposeStage(stage *CompletionStage, composedStage *CompletionStage)
	//OnCompleteGraph indicates that the graph is now finished and cannot be modified
	OnCompleteGraph()
}

// CompletionGraph describes the graph itself
type CompletionGraph struct {
	ID            string
	FunctionID    string
	stages        map[string]*CompletionStage
	eventListener CompletionEventListener
	log           *logrus.Entry
	committed     bool
	completed     bool
}

// New Creates a new graph
func New(id string, functionID string, listener CompletionEventListener) *CompletionGraph {
	return &CompletionGraph{
		ID:            id,
		FunctionID:    functionID,
		stages:        make(map[string]*CompletionStage),
		eventListener: listener,
		log:           log.WithFields(logrus.Fields{"graph_id": id, "function_id": functionID}),
		committed:     false,
		completed:     false,
	}
}

// IsCommitted Has the graph been marked as committed by HandleCommitted
func (graph *CompletionGraph) IsCommitted() bool {
	return graph.committed
}

// HandleCommitted Commits Graph  - this allows it to complete once all outstanding execution is finished
// When a graph is Committed stages may still be added but only only while at least one stage is executing
func (graph *CompletionGraph) HandleCommitted() {
	graph.log.Info("committing graph")
	graph.committed = true
	graph.checkForCompletion()
}

// IsCompleted indicates if the graph is completed
func (graph *CompletionGraph) IsCompleted() bool {
	return graph.completed
}

// HandleCompleted closes the graph for modifications
// this should only be called once an OnComplete event has been emmitted by the graph
func (graph *CompletionGraph) HandleCompleted() {
	graph.log.Info("completing graph")
	graph.completed = true
}

// GetStage gets a stage from the graph  returns nil if the stage isn't found
func (graph *CompletionGraph) GetStage(stageID string) *CompletionStage {
	return graph.stages[stageID]
}

// GetStages Get a list of stages by id, returns nil if any of the stages are not found
func (graph *CompletionGraph) GetStages(stageIDs []string) []*CompletionStage {
	res := make([]*CompletionStage, len(stageIDs))
	for i, id := range stageIDs {

		stage := graph.GetStage(id)
		if stage == nil {
			return nil
		}
		res[i] = stage
	}
	return res
}

// NextStageID Returns the next stage ID to use for nodes
func (graph *CompletionGraph) NextStageID() string {
	return strconv.Itoa(len(graph.stages))
}



// HandleStageAdded appends a stage into the graph updating the dpendencies of that stage
// It returns an error if the stage event is invalid, or if another stage exists with the same ID
func (graph *CompletionGraph) HandleStageAdded(event *model.StageAddedEvent, shouldTrigger bool) error {
	log := graph.log.WithFields(logrus.Fields{"stage": event.StageId, "op": event.Op})

	strategy, err := getStrategyFromOperation(event.Op)

	if err != nil {
		log.Errorf("invalid/unsupported operation %s", event.Op)
		return err
	}

	if graph.IsCompleted() {
		log.Errorf("Graph already completed")
		return fmt.Errorf("Graph already completed")
	}
	_, hasStage := graph.stages[event.StageId]

	if hasStage {
		log.Error("Duplicate stage")
		return fmt.Errorf("Duplicate stage %d", event.StageId)
	}
	log.Info("Adding stage to graph")
	node := &CompletionStage{
		ID:           event.StageId,
		operation:    event.Op,
		strategy:     strategy,
		closure:      event.Closure,
		whenComplete: make(chan bool),
		result:       nil,
		dependencies: graph.GetStages(event.Dependencies),
	}
	for _, dep := range node.dependencies {
		dep.children = append(dep.children, node)
	}
	graph.stages[node.ID] = node

	if shouldTrigger {
		graph.triggerReadyStages()
	}
	return nil
}

// HandleStageCompleted Indicates that a stage completion event has been processed and may be incorporated into the graph
// This should be called when recovering a graph or within an OnStageCompleted event
func (graph *CompletionGraph) HandleStageCompleted(event *model.StageCompletedEvent, shouldTrigger bool) bool {
	log := graph.log.WithFields(logrus.Fields{"success": event.Result.Successful, "stage_id": event.StageId})
	log.Info("Completing node")
	node := graph.stages[event.StageId]
	success := node.complete(event.Result)

	if node.composeReference != nil {
		graph.tryCompleteComposedStage(node.composeReference, node)
	}

	if shouldTrigger {
		graph.triggerReadyStages()
	}

	graph.checkForCompletion()

	return success
}

// HandleInvokeComplete is signaled when an invocation (or function call) associated with a stage is completed
// This may signal completion of the stage (in which case a Complete Event is raised)
func (graph *CompletionGraph) HandleInvokeComplete(stageID string, result *model.CompletionResult) {
	log.WithField("stage_id", stageID).Info("Completing stage with faas response")
	stage := graph.stages[stageID]
	stage.handleResult(graph, result)
}

// HandleStageComposed handles a compose nodes event  - this should be called on graph recovery,
// or within an OnComposeStage event
func (graph *CompletionGraph) HandleStageComposed(event *model.StageComposedEvent) {
	log := graph.log.WithFields(logrus.Fields{"stage": event.StageId, "composed_stage": event.ComposedStageId})
	log.Info("Setting composed reference")
	outer := graph.stages[event.StageId]
	inner := graph.stages[event.ComposedStageId]
	inner.composeReference = outer

	// If inner stage is complete, complete outer stage too
	graph.tryCompleteComposedStage(outer, inner)
}

// Recover Trigger recovers of any pending nodes in the graph
// any triggered nodes that were pending when the graph is active will be failed with a stage recovery failed error
func (graph *CompletionGraph) Recover() {
	for _, stage := range graph.stages {
		if !stage.IsResolved() {
			triggered, _, _ := stage.requestTrigger()
			if triggered {
				graph.log.WithFields(logrus.Fields{"stage": stage.ID}).Info("Failing irrecoverable node")
				graph.eventListener.OnCompleteStage(stage, stageRecoveryFailedResult)
			}
		}
	}

	if len(graph.stages) > 0 {
		graph.log.Info("Retrieved stages from storage")
	}

	graph.checkForCompletion()
}

var stageRecoveryFailedResult = internalErrorResult(model.ErrorDatumType_stage_lost, "Stage invocation lost - stage may still be executing")

func (graph *CompletionGraph) tryCompleteComposedStage(outer *CompletionStage, inner *CompletionStage) {
	if inner.IsResolved() && !outer.IsResolved() {
		graph.log.WithFields(logrus.Fields{"stage": outer.ID, "inner_stage": inner.ID}).Info("Completing composed stage with inner stage")
		graph.eventListener.OnCompleteStage(outer, inner.result)
	}
}

func (graph *CompletionGraph) triggerReadyStages() {
	for _, stage := range graph.stages {
		if !stage.IsResolved() {
			triggered, status, input := stage.requestTrigger()
			if triggered {
				log := graph.log.WithFields(logrus.Fields{"stage_id": stage.ID, "status": status})
				log.Info("Preparing to trigger stage")
				stage.trigger(status, graph.eventListener, input)

			}
		}
	}
}

func (graph *CompletionGraph) getPendingCount() uint32 {
	count := uint32(0)
	for _, s := range graph.stages {
		if !s.IsResolved() {
			count++
		}
	}
	return count
}

func (graph *CompletionGraph) checkForCompletion() {
	pendingCount := graph.getPendingCount()
	if graph.IsCommitted() && !graph.IsCompleted() && pendingCount == 0 {
		graph.log.Info("Graph successfully completed")
		graph.eventListener.OnCompleteGraph()
	} else {
		graph.log.WithFields(logrus.Fields{"pending": pendingCount}).Info("Pending executions before graph can be completed")
	}
}
