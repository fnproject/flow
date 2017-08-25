// Package graph describes a persistent, reliable completion graph for a process
package graph

import (
	"fmt"
	"reflect"
	"strconv"
	"time"

	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/fnproject/completer/model"
	"github.com/sirupsen/logrus"
)

var log = logrus.WithField("logger", "graph")

const (
	maxDelaySeconds = 900
)

// CompletionEventListener is a callback interface to receive notifications about stage triggers and graph events
type CompletionEventListener interface {
	//OnExecuteStage indicates that  a stage is due to be executed with the given arguments
	OnExecuteStage(stage *CompletionStage, results []*model.CompletionResult)
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

func (graph *CompletionGraph) GetStages() []*CompletionStage {
	stages := make([]*CompletionStage, 0, len(graph.stages))

	for _, stage := range graph.stages {
		stages = append(stages, stage)
	}
	return stages
}

// IsCommitted Has the graph been marked as committed by HandleCommitted
func (graph *CompletionGraph) IsCommitted() bool {
	return graph.committed
}

// HandleCommitted Commits Graph  - this allows it to complete once all outstanding execution is finished
// When a graph is Committed stages may still be added but only only while at least one stage is executing
func (graph *CompletionGraph) handleCommitted() {
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
func (graph *CompletionGraph) handleCompleted() {
	graph.log.Info("completing graph")
	graph.completed = true
}

// GetStage gets a stage from the graph  returns nil if the stage isn't found
func (graph *CompletionGraph) GetStage(stageID string) *CompletionStage {
	return graph.stages[stageID]
}

// NextStageID Returns the next stage ID to use for nodes
func (graph *CompletionGraph) NextStageID() string {
	return strconv.Itoa(len(graph.stages))
}

// HandleStageAdded appends a stage into the graph updating the dpendencies of that stage
// It returns an error if the stage event is invalid, or if another stage exists with the same ID
func (graph *CompletionGraph) handleStageAdded(event *model.StageAddedEvent, shouldTrigger bool) {
	log := graph.log.WithFields(logrus.Fields{"stage": event.StageId, "op": event.Op})

	strategy, err := getStrategyFromOperation(event.Op)
	if err != nil {
		panic(fmt.Sprintf("invalid/unsupported operation %s", event.Op))
	}

	if len(event.Dependencies) < strategy.MinDependencies ||
		strategy.MaxDependencies >= 0 && len(event.Dependencies) > strategy.MaxDependencies {
		msg := fmt.Sprintf("Invalid no. of dependencies for operation %s, max %d, min %d, got %d",
			event.Op.String(), strategy.MaxDependencies, strategy.MinDependencies, len(event.Dependencies))
		panic(msg)
	}

	if _, hasStage := graph.stages[event.StageId]; hasStage {
		panic(fmt.Errorf("Duplicate stage %s", event.StageId))
	}

	depStages := make([]*CompletionStage, len(event.Dependencies))

	for i, id := range event.Dependencies {
		stage := graph.GetStage(id)
		if stage == nil {
			panic(fmt.Sprintf("Dependent stage %s not found", id))
		}
		depStages[i] = stage
	}

	log.Info("Adding stage to graph")
	node := &CompletionStage{
		ID:           event.StageId,
		operation:    event.Op,
		strategy:     strategy,
		closure:      event.Closure,
		whenComplete: actor.NewFuture(10000 * time.Hour), // can take indefinitely long
		result:       nil,
		dependencies: depStages,
	}
	for _, dep := range depStages {
		dep.children = append(dep.children, node)
	}
	graph.stages[node.ID] = node

	if shouldTrigger {
		graph.triggerReadyStages()
	}
}

// handleStageCompleted Indicates that a stage completion event has been processed and may be incorporated into the graph
// This should be called when recovering a graph or within an OnStageCompleted event
func (graph *CompletionGraph) handleStageCompleted(event *model.StageCompletedEvent, shouldTrigger bool) bool {
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

// handleInvokeComplete is signaled when an invocation (or function call) associated with a stage is completed
// This may signal completion of the stage (in which case a Complete Event is raised)
func (graph *CompletionGraph) handleInvokeComplete(event *model.FaasInvocationCompletedEvent) {
	log.WithField("stage_id", event.StageId).Info("Completing stage with faas response")
	stage := graph.stages[event.StageId]
	stage.handleResult(graph, event.Result)
}

// handleStageComposed handles a compose nodes event  - this should be called on graph recovery,
// or within an OnComposeStage event
func (graph *CompletionGraph) handleStageComposed(event *model.StageComposedEvent) {
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

var stageRecoveryFailedResult = model.NewInternalErrorResult(model.ErrorDatumType_stage_lost, "Stage invocation lost - stage may still be executing")

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
		if pendingCount > 0 {
			graph.log.WithFields(logrus.Fields{"pending": pendingCount}).Info("Pending executions before graph can be completed")
		}
	}
}

// UpdateWithEvent updates this graph's state according to the received event
func (graph *CompletionGraph) UpdateWithEvent(event model.Event, mayTrigger bool) {
	switch e := event.(type) {
	case *model.GraphCommittedEvent:
		graph.handleCommitted()

	case *model.GraphCompletedEvent:
		graph.handleCompleted()

	case *model.StageAddedEvent:
		graph.handleStageAdded(e, mayTrigger)

	case *model.StageCompletedEvent:
		graph.handleStageCompleted(e, mayTrigger)

	case *model.FaasInvocationCompletedEvent:
		graph.handleInvokeComplete(e)

	case *model.StageComposedEvent:
		graph.handleStageComposed(e)

	default:
		graph.log.Warnf("Ignoring event of unknown type %v", reflect.TypeOf(e))
	}
}

// ValidateCommand validates whether the given command can be correctly applied to the current graph's state
func (graph *CompletionGraph) ValidateCommand(cmd model.Command) model.ValidationError {
	// disallow graph structural changes when complete
	if addCmd, ok := cmd.(model.AddStageCommand); ok {
		if graph.IsCompleted() {
			return model.NewGraphCompletedError(addCmd.GetGraphId())
		}
		strategy, err := getStrategyFromOperation(addCmd.GetOperation())
		if err != nil {
			return model.NewInvalidOperationError(addCmd.GetGraphId())
		}

		if addCmd.GetDependencyCount() < strategy.MinDependencies ||
			strategy.MaxDependencies >= 0 && addCmd.GetDependencyCount() > strategy.MaxDependencies {
			return model.NewInvalidStageDependenciesError(addCmd.GetGraphId())
		}
	}

	// Then do individual checks dependent on type
	switch msg := cmd.(type) {
	case *model.AddDelayStageRequest:
		if msg.DelayMs <= 0 || msg.DelayMs > maxDelaySeconds*1000 {
			return model.NewInvalidDelayError(msg.GraphId, msg.DelayMs)
		}

	case *model.AddChainedStageRequest:
		if valid := graph.validateStages(msg.Deps); !valid {
			return model.NewInvalidStageDependenciesError(msg.GraphId)
		}

	case *model.CompleteStageExternallyRequest:
		stage := graph.GetStage(msg.StageId)
		if stage == nil {
			return model.NewStageNotFoundError(msg.GraphId, msg.StageId)
		}
		if stage.GetOperation() != model.CompletionOperation_externalCompletion {
			return model.NewStageNotCompletableError(msg.GraphId, msg.StageId)
		}

	case *model.GetStageResultRequest:
		if valid := graph.validateStages(append(make([]string, 0), msg.StageId)); !valid {
			return model.NewStageNotFoundError(msg.GraphId, msg.StageId)
		}
	}

	return nil
}

// Validate a list of stages. If any of them is missing, returns false
func (graph *CompletionGraph) validateStages(stageIDs []string) bool {
	for _, stage := range stageIDs {
		if graph.GetStage(stage) == nil {
			return false
		}
	}
	return true
}
