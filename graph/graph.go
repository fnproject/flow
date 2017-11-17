// Package graph describes a persistent, reliable completion graph for a process
package graph

import (
	"fmt"
	"reflect"
	"strconv"
	"time"

	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/fnproject/flow/model"
	"github.com/sirupsen/logrus"
)

var log = logrus.WithField("logger", "graph")

const (
	maxDelaySeconds      = 86400
	maxStages            = 1000
	maxTerminationHooks  = 100
	stageCallbackTimeout = 10000 * time.Hour // wait indefinitely
)

// CompletionEventListener is a callback interface to receive notifications about stage triggers and graph events
type CompletionEventListener interface {
	//OnExecuteStage indicates that  a stage is due to be executed with the given arguments
	OnExecuteStage(stage *CompletionStage, results []*model.CompletionResult)
	//OnCompleteStage indicates that a stage is finished and its result is available
	OnCompleteStage(stage *CompletionStage, result *model.CompletionResult)
	//OnCompose Stage indicates that another stage should be composed into this one
	OnComposeStage(stage *CompletionStage, composedStage *CompletionStage)
	//OnGraphExecutionFinished indicates that the graph is now finished executing all user stages and cannot be structurally modified
	OnGraphExecutionFinished()
	//OnGraphComplete indicates that the graph has completed processing and can be safely passivated
	OnGraphComplete()
}

// State describes the lifecycle state of graph
type State string

const (
	// StateInitial : The graph has been created but not yet committed, can accept changes
	StateInitial = State("initial")
	// StateCommitted : the graph has been committed and can now terminate and can accept changes
	StateCommitted = State("committed")
	// StateTerminating : No more regular stage executions
	StateTerminating = State("terminating")
	// StateCompleted : No more events/data
	StateCompleted = State("completed")
)

// CompletionGraph describes the graph itself
type CompletionGraph struct {
	ID            string
	FunctionID    string
	stages        map[string]*CompletionStage
	eventListener CompletionEventListener
	log           *logrus.Entry
	state         State
	// This is a meta-stage (does not appear in the graph) that acts as the source for the termination chain
	// This is completed with the termination state
	terminationRoot      *RawDependency
	terminationChainHead *CompletionStage
}

// New Creates a new graph
func New(id string, functionID string, listener CompletionEventListener) *CompletionGraph {
	return &CompletionGraph{
		ID:              id,
		FunctionID:      functionID,
		stages:          make(map[string]*CompletionStage),
		eventListener:   listener,
		log:             log.WithFields(logrus.Fields{"graph_id": id, "function_id": functionID}),
		state:           StateInitial,
		terminationRoot: &RawDependency{ID: "_termination"},
	}
}

// GetStages gets a copy of the list of current  stages of the graph - the the stages them selves are the live objects   (and may vary after other events are processed)
func (graph *CompletionGraph) GetStages() []*CompletionStage {
	stages := make([]*CompletionStage, 0, len(graph.stages))

	for _, stage := range graph.stages {
		stages = append(stages, stage)
	}
	return stages
}

// IsCommitted Has the graph been marked as committed by HandleCommitted
func (graph *CompletionGraph) IsCommitted() bool {
	switch graph.state {
	case StateInitial:
		return false
	default:
		return true
	}
}

// HandleCommitted Commits Graph  - this allows it to complete once all outstanding execution is finished
// When a graph is Committed stages may still be added but only only while at least one stage is executing
func (graph *CompletionGraph) handleCommitted() {
	graph.log.Info("committing graph")
	graph.state = StateCommitted
	graph.checkForExecutionFinished()
}

// IsCompleted indicates if the graph is completed
func (graph *CompletionGraph) IsCompleted() bool {
	return graph.state == StateCompleted
}

// HandleCompleted closes the graph for modifications
// this should only be called once an OnComplete event has been emmitted by the graph
func (graph *CompletionGraph) handleCompleted() {
	graph.log.Info("completing graph")
	graph.state = StateCompleted
}

// handleTerminating completes the termination stage with a specified state,
// this will start triggering termination hooks if any are registered
func (graph *CompletionGraph) handleTerminating(event *model.GraphTerminatingEvent, shouldTrigger bool) {
	graph.log.Info("Graph terminating")
	graph.state = StateTerminating

	graph.terminationRoot.SetResult(model.NewSuccessfulResult(model.NewStateDatum(event.State)))

	if shouldTrigger {
		graph.triggerReadyStages()
	}
	graph.checkForExecutionFinished()

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
	deps := make([]StageDependency, len(event.Dependencies))

	for i, id := range event.Dependencies {
		stage := graph.GetStage(id)
		if stage == nil {
			panic(fmt.Sprintf("Dependent stage %s not found", id))
		}
		depStages[i] = stage
		deps[i] = stage
	}

	log.Info("Adding stage to graph")
	stage := &CompletionStage{
		ID:           event.StageId,
		operation:    event.Op,
		strategy:     strategy,
		closure:      event.Closure,
		whenComplete: actor.NewFuture(stageCallbackTimeout),
		dependencies: deps,
		execPhase:    MainExecPhase,
	}
	for _, dep := range depStages {
		dep.children = append(dep.children, stage)
	}
	graph.stages[stage.ID] = stage

	if shouldTrigger {
		graph.triggerReadyStages()
	}
}

func (graph *CompletionGraph) handleAddTerminationHook(event *model.StageAddedEvent) {
	log.WithField("stage_id", event.StageId).Info("Adding termination hook")
	strategy, err := getStrategyFromOperation(model.CompletionOperation_terminationHook)
	if err != nil {
		panic(fmt.Sprintf("invalid/unsupported operation %s", event.Op))
	}

	stage := &CompletionStage{
		ID:           event.StageId,
		operation:    model.CompletionOperation_terminationHook,
		strategy:     strategy,
		closure:      event.Closure,
		dependencies: []StageDependency{graph.terminationRoot},
		whenComplete: actor.NewFuture(10000 * time.Hour), // can take indefinitely long
		execPhase:    TerminationExecPhase,
	}

	if graph.terminationChainHead != nil {
		oldHead := graph.terminationChainHead
		stage.children = []*CompletionStage{oldHead}
		oldHead.dependencies = []StageDependency{stage}
	}
	graph.terminationChainHead = stage

	graph.stages[stage.ID] = stage

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

	graph.checkForExecutionFinished()

	return success
}

// handleInvokeComplete is signaled when an invocation (or function call) associated with a stage is completed
// This may signal completion of the stage (in which case a Complete Event is raised)
func (graph *CompletionGraph) handleInvokeComplete(event *model.FaasInvocationCompletedEvent) {
	log.WithField("fn_call_id", event.CallId).WithField("stage_id", event.StageId).Info("Completing stage with faas response")
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

	graph.triggerReadyStages()
	graph.checkForExecutionFinished()
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

func (graph *CompletionGraph) getPendingCount(phase ExecutionPhase) uint32 {
	count := uint32(0)
	for _, s := range graph.stages {
		if s.execPhase == phase && !s.IsResolved() {
			count++
		}
	}
	return count
}

func (graph *CompletionGraph) canModifyGraph() bool {
	switch graph.state {
	case StateInitial, StateCommitted:
		return true
	default:
		return false
	}
}

func (graph *CompletionGraph) checkForExecutionFinished() {

	switch graph.state {
	case StateCommitted:
		pendingCount := graph.getPendingCount(MainExecPhase)
		if pendingCount == 0 {
			graph.eventListener.OnGraphExecutionFinished()
		} else {
			graph.log.WithFields(logrus.Fields{"pending": pendingCount}).Info("Pending executions before graph can terminate")
		}
	case StateTerminating:
		pendingCount := graph.getPendingCount(TerminationExecPhase)
		if pendingCount == 0 {
			graph.eventListener.OnGraphComplete()
		} else {
			graph.log.WithFields(logrus.Fields{"pending": pendingCount}).Info("Pending shutdown stages before graph can be completed")
		}

	case StateInitial, StateCompleted:
		// nop
	}

}

// UpdateWithEvent updates this graph's state acGetGraphStateRequestcording to the received event
func (graph *CompletionGraph) UpdateWithEvent(event model.Event, mayTrigger bool) {
	switch e := event.(type) {
	case *model.GraphCommittedEvent:
		graph.handleCommitted()

	case *model.GraphTerminatingEvent:
		graph.handleTerminating(e, mayTrigger)

	case *model.GraphCompletedEvent:
		graph.handleCompleted()

	case *model.StageAddedEvent:
		{
			if e.Op == model.CompletionOperation_terminationHook {
				graph.handleAddTerminationHook(e)
			} else {
				graph.handleStageAdded(e, mayTrigger)
			}

		}

	case *model.StageCompletedEvent:
		graph.handleStageCompleted(e, mayTrigger)

	case *model.FaasInvocationCompletedEvent:
		if mayTrigger {
			graph.handleInvokeComplete(e)
		}

	case *model.StageComposedEvent:
		graph.handleStageComposed(e)

	case *model.FaasInvocationStartedEvent:
		// NOOP

	default:
		graph.log.Warnf("Ignoring event of unknown type %v", reflect.TypeOf(e))
	}
}

// ValidateCommand validates whether the given command can be correctly applied to the current graph's state
func (graph *CompletionGraph) ValidateCommand(cmd model.Command) model.ValidationError {
	// disallow graph structural changes when complete
	if addCmd, ok := cmd.(model.AddStageCommand); ok {
		if !graph.canModifyGraph() {
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

		if len(graph.stages) >= maxStages {
			return model.NewTooManyStagesError(addCmd.GetGraphId())
		}

		if addCmd.GetOperation() == model.CompletionOperation_terminationHook {
			var hookCount int
			for _, s := range graph.stages {
				if s.GetOperation() == model.CompletionOperation_terminationHook {
					hookCount++
				}
			}
			if hookCount >= maxTerminationHooks {
				return model.NewTooManyTerminationHooksError(addCmd.GetGraphId())
			}
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
