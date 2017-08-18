package graph

import (
	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/fnproject/completer/model"
)

// CompletionStage is a node in  Graph
type CompletionStage struct {
	ID        string
	operation model.CompletionOperation
	strategy  strategy
	// optional closure
	closure      *model.BlobDatum
	result       *model.CompletionResult
	dependencies []*CompletionStage
	// Composed children
	children     []*CompletionStage
	whenComplete *actor.Future
	// Parent stage if I'm a child  - this is what I complete when I'm done
	composeReference *CompletionStage
	// this only prevents a a stage from triggering twice in the same generation
	triggered bool
}

// GetOperation returns the operation for this stage
func (stage *CompletionStage) GetOperation() model.CompletionOperation {
	return stage.operation
}

// GetClosure returns the closure for this stage
func (stage *CompletionStage) GetClosure() *model.BlobDatum {
	return stage.closure
}

// GetResult returns this stage's result if available
func (stage *CompletionStage) GetResult() *model.CompletionResult {
	return stage.result
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
