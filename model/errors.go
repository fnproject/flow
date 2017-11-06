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

// NewGraphCreationError : failed to create graph, no graph exists afterwards
func NewGraphCreationError(graphID string) ValidationError {
	return &InvalidGraphOperation{GraphId: graphID, Err: "Graph creation failed"}
}

// NewGraphNotFoundError : anywhere a graph is not fonud
func NewGraphNotFoundError(graphID string) ValidationError {
	return &InvalidGraphOperation{GraphId: graphID, Err: "Graph not found"}
}

// NewGraphCompletedError : indicates an invalid operation on an already completed (or terminating) graph
func NewGraphCompletedError(graphID string) ValidationError {
	return &InvalidGraphOperation{GraphId: graphID, Err: "Graph already completed"}
}

// NewInvalidDelayError - something wasn't right with your delay
func NewInvalidDelayError(graphID string, delayMs int64) ValidationError {
	return &InvalidGraphOperation{GraphId: graphID, Err: fmt.Sprintf("Invalid delay stage of %d milliseconds", delayMs)}
}

// NewStageNotFoundError : anywhere a stage on an existing graph was not found
func NewStageNotFoundError(graphID string, stageID string) ValidationError {
	return &InvalidStageOperation{GraphId: graphID, StageId: stageID, Err: "Stage not found"}
}

// NewAwaitStageError : Error (including a timeout) awaiting a stage
func NewAwaitStageError(graphID string, stageID string) ValidationError {
	return &InvalidStageOperation{GraphId: graphID, StageId: stageID, Err: "Stage failed to complete"}
}

// NewStageNotCompletableError : Stage is not externally completable
func NewStageNotCompletableError(graphID string, stageID string) ValidationError {
	return &InvalidStageOperation{GraphId: graphID, StageId: stageID, Err: "Stage not completable externally"}
}

// NewInvalidStageDependenciesError : bad stage deps in request
func NewInvalidStageDependenciesError(graphID string) ValidationError {
	return &InvalidGraphOperation{GraphId: graphID, Err: "Failed to create stage with invalid dependencies"}
}

// NewInvalidOperationError : bad operation in request
func NewInvalidOperationError(graphID string) ValidationError {
	return &InvalidGraphOperation{GraphId: graphID, Err: "Invalid stage operation"}
}

// NewTooManyStagesError : too many stages in your graph
func NewTooManyStagesError(graphID string) ValidationError {
	return &InvalidGraphOperation{GraphId: graphID, Err: "Failed to add stage, graph has maximum number of stages"}
}

// NewTooManyTerminationHooksError : too many termination hooks on your graph
func NewTooManyTerminationHooksError(graphID string) ValidationError {
	return &InvalidGraphOperation{GraphId: graphID, Err: "Failed to add termination hook, graph has maximum number of termination hooks"}
}
