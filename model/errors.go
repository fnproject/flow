package model

import (
	"fmt"

	proto "github.com/golang/protobuf/proto"
)

// ValidationError is the base interface for validation error messages that can be returned from graph actors
type ValidationError interface {
	error
	proto.Message
}

func NewGraphCreationError(graphId string) ValidationError {
	return &InvalidGraphOperation{GraphId: graphId, Err: "Graph creation failed"}
}

func NewGraphNotFoundError(graphId string) ValidationError {
	return &InvalidGraphOperation{GraphId: graphId, Err: "Graph not found"}
}

func NewGraphCompletedError(graphId string) ValidationError {
	return &InvalidGraphOperation{GraphId: graphId, Err: "Graph already completed"}
}

func NewInvalidDelayError(graphId string, delayMs int64) ValidationError {
	return &InvalidGraphOperation{GraphId: graphId, Err: fmt.Sprintf("Invalid delay stage of %d milliseconds", delayMs)}
}

func NewStageNotFoundError(graphId string, stageId string) ValidationError {
	return &InvalidStageOperation{GraphId: graphId, StageId: stageId, Err: "Stage not found"}
}

func NewStageCompletionError(graphId string, stageId string) ValidationError {
	return &InvalidStageOperation{GraphId: graphId, StageId: stageId, Err: "Stage failed to complete"}
}

func NewStageNotCompletableError(graphId string, stageId string) ValidationError {
	return &InvalidStageOperation{GraphId: graphId, StageId: stageId, Err: "Stage not completable externally"}
}

func NewInvalidStageDependenciesError(graphId string) ValidationError {
	return &InvalidGraphOperation{GraphId: graphId, Err: "Failed to create stage with invalid dependencies"}
}

func NewInvalidOperationError(graphId string) ValidationError {
	return &InvalidGraphOperation{GraphId: graphId, Err: "Invalid stage operation"}
}

func NewFailedToRegisterCallback(graphId string) ValidationError {
	return &InvalidGraphOperation{GraphId: graphId, Err: "Failed to add termination hook"}
}
