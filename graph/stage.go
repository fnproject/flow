package graph

import (
	"github.com/fnproject/completer/model"
)

// Completion Stage

type CompletionStage struct {
	Id        StageId
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

func (stage *CompletionStage) complete(result *model.CompletionResult) bool {
	stage.triggered = true
	if stage.result == nil {
		stage.result = result
		close(stage.whenComplete)
		return true
	}
	return false
}

func (stage *CompletionStage) isResolved() bool {
	return stage.result != nil
}

func (stage *CompletionStage) isSuccessful() bool {
	return stage.isResolved() && stage.result.Status == model.ResultStatus_succeeded
}

func (stage *CompletionStage) isFailed() bool {
	return stage.isResolved() && (stage.result.Status == model.ResultStatus_failed )
}

// determines if the node can be triggered and returns the trigger type, and pending result if it can be
func (stage *CompletionStage) requestTrigger() (satisfied bool, status TriggerStatus, satisfyingResults []*model.CompletionResult) {
	if stage.triggered {
		// never trigger a triggered stage
		return false, TriggerStatus_failed, nil

	} else {
		stage.triggered = true
		return stage.strategy.TriggerStrategy(stage.dependencies)

	}
}
