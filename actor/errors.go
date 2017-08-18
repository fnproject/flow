package actor

import (
	"fmt"

	"github.com/fnproject/completer/model"
)

func NewGraphCreationError(graphId string) *model.InvalidGraphOperation {
	return &model.InvalidGraphOperation{GraphId: graphId, Err: "Graph creation failed"}
}

func NewGraphNotFoundError(graphId string) *model.InvalidGraphOperation {
	return &model.InvalidGraphOperation{GraphId: graphId, Err: "Graph not found"}
}


func NewGraphCompletedError(graphId string) *model.InvalidGraphOperation {
	return &model.InvalidGraphOperation{GraphId: graphId, Err: "Graph already completed"}
}

func NewInvalidDelayError(graphId string, delayMs int64) *model.InvalidGraphOperation {
	return &model.InvalidGraphOperation{GraphId: graphId, Err: fmt.Sprintf("Invalid delay stage of %d milliseconds", delayMs)}
}

func NewStageNotFoundError(graphId string, stageId string) *model.InvalidStageOperation {
	return &model.InvalidStageOperation{GraphId: graphId, StageId: stageId, Err: "Stage not found"}
}

func NewStageCompletionError(graphId string, stageId string) *model.InvalidStageOperation {
	return &model.InvalidStageOperation{GraphId: graphId, StageId: stageId, Err: "Stage failed to complete"}
}

func NewStageNotCompletableError(graphId string, stageId string) *model.InvalidStageOperation {
	return &model.InvalidStageOperation{GraphId: graphId, StageId: stageId, Err: "Stage not completable externally"}
}
