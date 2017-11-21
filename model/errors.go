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
func NewGraphCreationError(flowID string) ValidationError {
	return &InvalidGraphOperation{FlowId: flowID, Err: "Graph creation failed"}
}

// NewGraphNotFoundError : anywhere a graph is not fonud
func NewGraphNotFoundError(flowID string) ValidationError {
	return &InvalidGraphOperation{FlowId: flowID, Err: "Graph not found"}
}

// NewGraphCompletedError : indicates an invalid operation on an already completed (or terminating) graph
func NewGraphCompletedError(flowID string) ValidationError {
	return &InvalidGraphOperation{FlowId: flowID, Err: "Graph already completed"}
}

// NewInvalidDelayError - something wasn't right with your delay
func NewInvalidDelayError(flowID string, delayMs int64) ValidationError {
	return &InvalidGraphOperation{FlowId: flowID, Err: fmt.Sprintf("Invalid delay stage of %d milliseconds", delayMs)}
}

// NewStageNotFoundError : anywhere a stage on an existing graph was not found
func NewStageNotFoundError(flowID string, stageID string) ValidationError {
	return &InvalidStageOperation{FlowId: flowID, StageId: stageID, Err: "Stage not found"}
}

// NewAwaitStageError : Error (including a timeout) awaiting a stage
func NewAwaitStageError(FlowID string, stageID string) ValidationError {
	return &InvalidStageOperation{FlowId: FlowID, StageId: stageID, Err: "Stage failed to complete"}
}

// NewInvalidStageDependenciesError : bad stage deps in request
func NewInvalidStageDependenciesError(flowID string) ValidationError {
	return &InvalidGraphOperation{FlowId: flowID, Err: "Failed to create stage with invalid dependencies"}
}

// NewInvalidOperationError : bad operation in request
func NewInvalidOperationError(flowID string) ValidationError {
	return &InvalidGraphOperation{FlowId: flowID, Err: "Invalid stage operation"}
}

// NewTooManyStagesError : too many stages in your graph
func NewTooManyStagesError(flowID string) ValidationError {
	return &InvalidGraphOperation{FlowId: flowID, Err: "Failed to add stage, graph has maximum number of stages"}
}

// NewTooManyTerminationHooksError : too many termination hooks on your graph
func NewTooManyTerminationHooksError(flowID string) ValidationError {
	return &InvalidGraphOperation{FlowId: flowID, Err: "Failed to add termination hook, graph has maximum number of termination hooks"}
}
