package graph

import (
	"github.com/fnproject/completer/model"
)

// Completion Stage

type CompletionStage struct {
	Id           StageId
	operation    model.CompletionOperation
	strategy     strategy
	// optional closure
	closure      *model.BlobDatum
	result       *model.CompletionResult
	dependencies []*CompletionStage
	// Composed children
	children     []*CompletionStage
	// TODO "when complete" future
	complete chan bool
	// Parent stage if I'm a child  - this is what I complete when I'm done
	composeReference *CompletionStage
}

func (stage *CompletionStage) Complete(result *model.CompletionResult) bool {
	if stage.result == nil {
		stage.result = result
		close(stage.complete)
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

// isSatisfied determines if the stage's trigger is satisfied, if so returns
// the list of results that were used to determine satisfaction
func (stage *CompletionStage) canTrigger() (satisfied bool, status TriggerStatus, satisfyingResults []*model.CompletionResult) {
	return stage.strategy.TriggerStrategy(stage)
}
