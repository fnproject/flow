package graph

import (
	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/fnproject/flow/model"
)

type ExecutionPhase string

const MainExecPhase = ExecutionPhase("main")
const TerminationExecPhase = ExecutionPhase("termination")

type StageDependency interface {
	GetID() string
	IsResolved() bool
	IsFailed() bool
	IsSuccessful() bool
	GetResult() *model.CompletionResult
}

// This is an input that obeys StageDependency
type RawDependency struct {
	ID     string
	result *model.CompletionResult
}

// CompletionStage is a node in  Graph
type CompletionStage struct {
	result    *model.CompletionResult
	ID        string
	operation model.CompletionOperation
	strategy  strategy
	// optional closure
	closure      *model.BlobDatum
	dependencies []StageDependency
	// Composed children
	children     []*CompletionStage
	whenComplete *actor.Future
	// Parent stage if I'm a child  - this is what I complete when I'm done
	composeReference *CompletionStage
	// this only prevents a a stage from triggering twice in the same generation
	triggered bool

	// When this node runs
	execPhase ExecutionPhase
}

func (stage *RawDependency) GetID() string {
	return stage.ID
}

func (r *RawDependency) GetResult() *model.CompletionResult {
	return r.result
}

func (r *RawDependency) IsResolved() bool {
	return r.result != nil
}

func (r *RawDependency) IsSuccessful() bool {
	return r.IsResolved() && r.result.Successful
}

func (r *RawDependency) IsFailed() bool {
	return r.IsResolved() && !r.result.Successful
}
func (r *RawDependency) SetResult(result *model.CompletionResult) {
	r.result = result
}

func (stage *CompletionStage) GetID() string {
	return stage.ID
}

// GetResult returns this stage's result if available
func (stage *CompletionStage) GetResult() *model.CompletionResult {
	return stage.result
}

// IsResolved is this stage resolved or pending
func (stage *CompletionStage) IsResolved() bool {
	return stage.result != nil
}

// IsSuccessful indicates if the stage was successful
func (stage *CompletionStage) IsSuccessful() bool {
	return stage.IsResolved() && stage.result.Successful
}

// IsFailed indicates if the stage failed
func (stage *CompletionStage) IsFailed() bool {
	return stage.IsResolved() && !stage.result.Successful
}

func (stage *CompletionStage) GetDeps() []StageDependency {
	return stage.dependencies
}

// GetOperation returns the operation for this stage
func (stage *CompletionStage) GetOperation() model.CompletionOperation {
	return stage.operation
}

// GetClosure returns the closure for this stage
func (stage *CompletionStage) GetClosure() *model.BlobDatum {
	return stage.closure
}

// WhenComplete returns a Future returning a *model.CompletionResult
func (stage *CompletionStage) WhenComplete() *actor.Future {
	return stage.whenComplete
}

func (stage *CompletionStage) complete(result *model.CompletionResult) bool {
	stage.triggered = true
	if stage.result == nil {
		stage.result = result
		stage.whenComplete.PID().Tell(result)
		return true
	}
	return false
}

func (stage *CompletionStage) IsTriggered() bool {
	return stage.IsResolved() || stage.triggered
}

// determines if the node can be triggered and returns the trigger type, and pending result if it can be
func (stage *CompletionStage) requestTrigger() (satisfied bool, status bool, satisfyingResults []*model.CompletionResult) {
	if stage.triggered {
		// never trigger a triggered stage
		return false, false, nil

	}

	triggered, status, results := stage.strategy.TriggerStrategy(stage.dependencies)

	if triggered {
		stage.triggered = true
	}
	return triggered, status, results

}

// execute is done, here is the result - go wild
func (stage *CompletionStage) handleResult(graph *CompletionGraph, result *model.CompletionResult) {
	stage.strategy.ResultHandlingStrategy(stage, graph, result)
}

// triggers the stage  this should produce some event on the listener that eventually updates the grah
func (stage *CompletionStage) trigger(status bool, listener CompletionEventListener, input []*model.CompletionResult) {
	var strategy ExecutionStrategy
	if status {
		strategy = stage.strategy.SuccessStrategy
	} else {
		strategy = stage.strategy.FailureStrategy
	}
	strategy(stage, listener, input)
}
