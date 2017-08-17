package actor

import (
	"fmt"

	"github.com/fnproject/completer/model"
)

func NewGraphCreationError(graphId string) *model.InvalidGraphOperation {
	return &model.InvalidGraphOperation{GraphId: graphId, Error: "Graph creation failed"}
}

func NewGraphNotFoundError(graphId string) *model.InvalidGraphOperation {
	return &model.InvalidGraphOperation{GraphId: graphId, Error: "Graph not found"}
}

func NewGraphEventPersistenceError(graphId string) *model.InvalidGraphOperation {
	return &model.InvalidGraphOperation{GraphId: graphId, Error: "Failed to persist event"}
}

func NewGraphCompletedError(graphId string) *model.InvalidGraphOperation {
	return &model.InvalidGraphOperation{GraphId: graphId, Error: "Graph already completed"}
}

func NewInvalidDelayError(graphId string, delayMs int64) *model.InvalidGraphOperation {
	return &model.InvalidGraphOperation{GraphId: graphId, Error: fmt.Sprintf("Invalid delay stage of %d milliseconds", delayMs)}
}

func NewStageNotFoundError(graphId string, stageId string) *model.InvalidStageOperation {
	return &model.InvalidStageOperation{GraphId: graphId, StageId: stageId, Error: "Stage not found"}
}

func NewStageCompletionError(graphId string, stageId string) *model.InvalidStageOperation {
	return &model.InvalidStageOperation{GraphId: graphId, StageId: stageId, Error: "Stage failed to complete"}
}

func NewStageNotCompletableError(graphId string, stageId string) *model.InvalidStageOperation {
	return &model.InvalidStageOperation{GraphId: graphId, StageId: stageId, Error: "Stage not completable externally"}
}
