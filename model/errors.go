package model

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// NewGraphCreationError : failed to create graph, no graph exists afterwards
func NewGraphCreationError(flowID string) error {
	return status.Errorf(codes.Internal, "Cannot spawn actor to manage flow: flow_id=%v", flowID)
}

// NewGraphAlreadyExistsError : can't create the same graph twice
func NewGraphAlreadyExistsError(flowID string) error {
	return status.Errorf(codes.AlreadyExists, "Flow already exists: flow_id=%v", flowID)
}

// NewGraphNotFoundError : anywhere a graph is not found
func NewGraphNotFoundError(flowID string) error {
	return status.Errorf(codes.NotFound, "Flow not found: flow_id=%v", flowID)
}

// NewGraphCompletedError : indicates an invalid operation on an already completed (or terminating) graph
func NewGraphCompletedError(flowID string) error {
	return status.Errorf(codes.FailedPrecondition, "Flow already completed: flow_id=%v", flowID)
}

// NewInvalidDelayError - something wasn't right with your delay
func NewInvalidDelayError(flowID string, delayMs int64) error {
	return status.Errorf(codes.InvalidArgument, "Invalid delay stage of %d milliseconds: flow_id=%v", delayMs, flowID)
}

// NewStageNotFoundError : anywhere a stage on an existing graph was not found
func NewStageNotFoundError(flowID string, stageID string) error {
	return status.Errorf(codes.NotFound, "Stage not found. flow_id=%v stage_id=%v", flowID, stageID)
}

// NewAwaitStageError : Error (including a timeout) awaiting a stage
func NewAwaitStageError(flowID string, stageID string) error {
	return status.Errorf(codes.Unknown, "Stage failed to complete: flow_id=%v stage_id=%v", flowID, stageID)
}

// NewInvalidStageDependenciesError : bad stage deps in request
func NewInvalidStageDependenciesError(flowID string) error {
	return status.Errorf(codes.InvalidArgument, "Failed to create stage with invalid dependencies: flow_id=%v", flowID)
}

// NewInvalidDatumError : request contains an invalid datum
func NewInvalidDatumError(flowID string) error {
	return status.Errorf(codes.InvalidArgument, "Invalid datum in command: flow_id=%v", flowID)
}

// NewNeedsClosureError :stage needs a closure
func NewNeedsClosureError(flowID string) error {
	return status.Errorf(codes.InvalidArgument, "Stage requires a closure but none was specified: flow_id=%v", flowID)
}

// NewShouldNotHaveClosureError :stage has a closure when it shouldn't
func NewShouldNotHaveClosureError(flowID string) error {
	return status.Errorf(codes.InvalidArgument, "Stage should not have  closure but one was specified: flow_id=%v", flowID)
}
// NewInvalidOperationError : bad operation in request
func NewInvalidOperationError(flowID string) error {
	return status.Errorf(codes.Unimplemented, "Invalid stage operation: flow_id=%v", flowID)
}

// NewTooManyStagesError : too many stages in your graph
func NewTooManyStagesError(flowID string) error {
	return status.Errorf(codes.ResourceExhausted, "Failed to add stage, flow has maximum number of stages: flow_id=%v", flowID)
}

// NewTooManyTerminationHooksError : too many termination hooks on your graph
func NewTooManyTerminationHooksError(flowID string) error {
	return status.Errorf(codes.ResourceExhausted, "Failed to add termination hook, flow has maximum number of termination hooks: flow_id=%v", flowID)
}
