package actor

import (
	"github.com/fnproject/completer/model"
	"fmt"
)

func NewGraphNotFoundError(graphId string) *model.InvalidGraphOperation {
	return &model.InvalidGraphOperation{GraphId: graphId, Error: "Graph not found"}
}

func NewGraphEventPersistenceError(graphId string) *model.InvalidGraphOperation {
	return &model.InvalidGraphOperation{GraphId: graphId, Error: "Failed to persist event"}
}

func NewGraphCompletedError(graphId string) *model.InvalidGraphOperation {
	return &model.InvalidGraphOperation{GraphId: graphId, Error: "Graph already completed"}
}

func NewInvalidDelayError(graphId string, delayMs uint64) *model.InvalidGraphOperation {
	return &model.InvalidGraphOperation{GraphId: graphId, Error: fmt.Sprintf("Invalid delay stage of %d milliseconds", delayMs)}
}

func NewStageNotFoundError(graphId string, stageId string) *model.InvalidStageOperation {
	return &model.InvalidStageOperation{GraphId: graphId, StageId: stageId, Error: "Stage not found"}
}

func NewStageNotCompletableError(graphId string, stageId string) *model.InvalidStageOperation {
	return &model.InvalidStageOperation{GraphId: graphId, StageId: stageId, Error: "Stage not completable externally"}
}
