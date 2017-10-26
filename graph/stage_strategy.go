package graph

import (
	"fmt"

	"github.com/fnproject/flow/model"
	"github.com/sirupsen/logrus"
)

// TriggerStrategy defines when a stage becomes active , and what the incoming status of the trigger is (success or fail)
// each type may or may not depend on the incoming dependencies of the node
type TriggerStrategy func(deps []StageDependency) (shouldTrigger bool, successfulTrigger bool, inputs []*model.CompletionResult)

//waitForAll marks node as succeeded if all are completed regardless of success or failure
func waitForAll(dependencies []StageDependency) (bool, bool, []*model.CompletionResult) {
	var results = make([]*model.CompletionResult, 0, len(dependencies))
	for _, s := range dependencies {
		if s.IsResolved() {
			results = append(results, s.GetResult())
		}
	}

	if len(results) == len(dependencies) {
		// if any dependency failed, this node should fail as well, and with only one result which is the first error
		for _, s := range dependencies {
			if s.IsFailed() {
				return true, false, []*model.CompletionResult{s.GetResult()}
			}
		}
		return true, true, results
	}

	return false, false, nil
}

// triggerAny marks a node as succeed if any one is resolved successfully,  or fails with the first error if all are failed
func triggerAny(dependencies []StageDependency) (bool, bool, []*model.CompletionResult) {
	var haveUnresolved bool
	var firstFailure StageDependency
	if 0 == len(dependencies) {
		// TODO: any({}) -  not clear if we should block here  or error
		return false, false, nil
	}
	for _, s := range dependencies {
		if s.IsResolved() {
			if !s.IsFailed() {
				return true, true, []*model.CompletionResult{s.GetResult()}
			}
			firstFailure = s

		} else {
			haveUnresolved = true
		}
	}
	if !haveUnresolved {
		return true, false, []*model.CompletionResult{firstFailure.GetResult()}
	}
	return false, false, nil
}

// triggerNever always marks the node as untriggered.
func triggerNever(stage []StageDependency) (bool, bool, []*model.CompletionResult) {
	return false, false, []*model.CompletionResult{}
}

// ExecutionStrategy defines how the node should behave when it is triggered
// (optionally calling the attached node closure and how the arguments to the closure are defined)
// different strategies can be applied when the stage completes successfully or exceptionally
type ExecutionStrategy func(stage *CompletionStage, listener CompletionEventListener, results []*model.CompletionResult)

// succeedWithEmpty triggers completion of the stage with an empty success
func succeedWithEmpty(stage *CompletionStage, listener CompletionEventListener, results []*model.CompletionResult) {
	listener.OnCompleteStage(stage, &model.CompletionResult{
		Successful: true,
		Datum:      model.NewEmptyDatum(),
	})
}

// invokeWithoutArgs triggers invoking the closure with no args
func invokeWithoutArgs(stage *CompletionStage, listener CompletionEventListener, results []*model.CompletionResult) {
	listener.OnExecuteStage(stage, []*model.CompletionResult{})
}

// invokeWithResult triggers invoking the closure with no args
func invokeWithResult(stage *CompletionStage, listener CompletionEventListener, results []*model.CompletionResult) {
	listener.OnExecuteStage(stage, results)
}

// invokeWithResultOrError triggers invoking the closure with a pair consisting of the  (result,<emtpy>) if the result is successful, or (<empty>,error) if the result is an error
// this can only be used with single-valued result
func invokeWithResultOrError(stage *CompletionStage, listener CompletionEventListener, results []*model.CompletionResult) {
	if len(results) != 1 {
		// TODO: Don't panic
		panic(fmt.Sprintf("Invalid state - tried to invoke single-valued stage %v with incorrect number of inputs %v", stage, results))
	}
	result := results[0]

	var args []*model.CompletionResult
	if result.Successful {
		args = []*model.CompletionResult{model.NewSuccessfulResult(result.Datum), model.NewEmptyResult()}
	} else {
		args = []*model.CompletionResult{model.NewEmptyResult(), model.NewFailedResult(result.Datum)}
	}

	listener.OnExecuteStage(stage, args)
}

// completeExternally is a noop
func completeExternally(stage *CompletionStage, listener CompletionEventListener, results []*model.CompletionResult) {
}

// propagateResult passes a single value from an upstream dependency to the stage result
func propagateResult(stage *CompletionStage, listener CompletionEventListener, results []*model.CompletionResult) {
	if len(results) != 1 {
		// TODO: Don't panic
		panic(fmt.Sprintf("Invalid state - tried to invoke single-valued stage %v with incorrect number of inputs %v", stage, results))
	}
	listener.OnCompleteStage(stage, results[0])
}

// ResultHandlingStrategy defines how the  result value of the stage is derived following execution of the stage
// if the result completes the node then implementations should signal to the graph that the node is complete (via graph.listener)
// this operation may moodify the graph (e.g. in a compose)
type ResultHandlingStrategy func(stage *CompletionStage, graph *CompletionGraph, stageInvokeResult *model.CompletionResult)

func invocationResult(stage *CompletionStage, graph *CompletionGraph, result *model.CompletionResult) {
	graph.eventListener.OnCompleteStage(stage, result)
}

func referencedStageResult(stage *CompletionStage, graph *CompletionGraph, result *model.CompletionResult) {

	if !result.Successful {
		// Errors fail the normal way
		graph.eventListener.OnCompleteStage(stage, result)
		return
	}
	// result must be a stageref
	if nil == result.Datum.GetStageRef() {
		//TODO complete stage with an error
		graph.eventListener.OnCompleteStage(stage, model.NewInternalErrorResult(model.ErrorDatumType_invalid_stage_response, "stage returned a non-stageref response"))
		return
	}
	refStage := graph.GetStage(result.Datum.GetStageRef().StageRef)
	if nil == refStage {
		graph.eventListener.OnCompleteStage(stage, model.NewInternalErrorResult(model.ErrorDatumType_invalid_stage_response, "referenced stage not found "))
		return
	}
	log.WithFields(logrus.Fields{"stage_id": stage.ID, "other_id": refStage.ID}).Info("Composing with new stage ")
	graph.eventListener.OnComposeStage(stage, refStage)
}

// parentStageResult completes the stage the the first parent's result
func parentStageResult(stage *CompletionStage, graph *CompletionGraph, _ *model.CompletionResult) {

	if len(stage.dependencies) != 1 {
		log.WithField("stage_id", stage.ID).Errorf("Invalid stage action  - trying to get parent result when stage does not have one parent got %d", len(stage.dependencies))
		panic("Trying to get a parent result with invalid parent deps")
	}
	if !stage.dependencies[0].IsResolved() {
		log.WithField("stage_id", stage.ID).Errorf("Invalid stage action  - trying to get parent result when stage parent has not completed ")
		panic("Trying to get a parent result with invalid parent deps")
	}

	graph.eventListener.OnCompleteStage(stage, stage.dependencies[0].GetResult())

}

func noResultStrategy(_ *CompletionStage, _ *CompletionGraph, _ *model.CompletionResult) {}

type strategy struct {
	CanAddChildren bool
	// -1 for unlimited
	MaxDependencies        int
	MinDependencies        int
	TriggerStrategy        TriggerStrategy
	SuccessStrategy        ExecutionStrategy
	FailureStrategy        ExecutionStrategy
	ResultHandlingStrategy ResultHandlingStrategy
}

func getStrategyFromOperation(operation model.CompletionOperation) (strategy, error) {
	switch operation {

	case model.CompletionOperation_acceptEither:
		return strategy{true, 2, 2, triggerAny, invokeWithResult, propagateResult, invocationResult}, nil

	case model.CompletionOperation_applyToEither:
		return strategy{true, 2, 2, triggerAny, invokeWithResult, propagateResult, invocationResult}, nil

	case model.CompletionOperation_thenAcceptBoth:
		return strategy{true, 2, 2, waitForAll, invokeWithResult, propagateResult, invocationResult}, nil

	case model.CompletionOperation_thenApply:
		return strategy{true, 1, 1, triggerAny, invokeWithResult, propagateResult, invocationResult}, nil

	case model.CompletionOperation_thenRun:
		return strategy{true, 1, 1, triggerAny, invokeWithoutArgs, propagateResult, invocationResult}, nil

	case model.CompletionOperation_thenAccept:
		return strategy{true, 1, 1, triggerAny, invokeWithResult, propagateResult, invocationResult}, nil

	case model.CompletionOperation_thenCompose:
		return strategy{true, 1, 1, triggerAny, invokeWithResult, propagateResult, referencedStageResult}, nil

	case model.CompletionOperation_thenCombine:
		return strategy{true, 2, 2, waitForAll, invokeWithResult, propagateResult, invocationResult}, nil

	case model.CompletionOperation_whenComplete:
		return strategy{true, 1, 1, triggerAny, invokeWithResultOrError, invokeWithResultOrError, parentStageResult}, nil

	case model.CompletionOperation_handle:
		return strategy{true, 1, 1, triggerAny, invokeWithResultOrError, invokeWithResultOrError, invocationResult}, nil

	case model.CompletionOperation_supply:
		return strategy{true, 0, 0, waitForAll, invokeWithoutArgs, propagateResult, invocationResult}, nil

	case model.CompletionOperation_invokeFunction:
		return strategy{true, 0, 0, waitForAll, completeExternally, completeExternally, invocationResult}, nil

	case model.CompletionOperation_completedValue:
		return strategy{true, 0, 0, triggerNever, completeExternally, propagateResult, noResultStrategy}, nil

	case model.CompletionOperation_delay:
		return strategy{true, 0, 0, triggerNever, completeExternally, completeExternally, noResultStrategy}, nil

	case model.CompletionOperation_allOf:
		return strategy{true, -1, 0, waitForAll, succeedWithEmpty, propagateResult, noResultStrategy}, nil

	case model.CompletionOperation_anyOf:
		return strategy{true, -1, 1, triggerAny, propagateResult, propagateResult, noResultStrategy}, nil

	case model.CompletionOperation_externalCompletion:
		return strategy{true, 0, 0, triggerNever, completeExternally, completeExternally, noResultStrategy}, nil

	case model.CompletionOperation_exceptionally:
		return strategy{true, 1, 1, triggerAny, propagateResult, invokeWithResult, invocationResult}, nil

	case model.CompletionOperation_terminationHook:
		return strategy{false, 0, 0, waitForAll, invokeWithResult, invokeWithResult, parentStageResult}, nil

	case model.CompletionOperation_exceptionallyCompose:
		return strategy{true, 1, 1, triggerAny, propagateResult, invokeWithResult, referencedStageResult}, nil

	default:
		return strategy{}, fmt.Errorf("Unrecognised operation %s", operation)
	}
}
