package graph

import (
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
	children []*CompletionStage
	// TODO "when complete" future
	whenComplete chan bool
	// Parent stage if I'm a child  - this is what I complete when I'm done
	composeReference *CompletionStage
	// this only prevents a a stage from triggering twice in the same generation
	triggered bool
}

func (stage *CompletionStage) GetOperation() model.CompletionOperation {
	return stage.operation
}

func (stage *CompletionStage) GetClosure() *model.BlobDatum {
	return stage.closure
}

func (stage *CompletionStage) complete(result *model.CompletionResult) bool {
	stage.triggered = true
	if stage.result == nil {
		stage.result = result
		close(stage.whenComplete)
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
	stage.triggered = true
	return stage.strategy.TriggerStrategy(stage.dependencies)
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
