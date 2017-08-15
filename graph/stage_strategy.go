package graph

import (
	"github.com/fnproject/completer/model"
	"fmt"
	"github.com/sirupsen/logrus"
)

// TriggerStrategy defines when a stage becomes active , and what the incoming status of the trigger is (success or fail)
// each type may or may not depend on the incoming dependencies of the node
type TriggerStrategy func(deps []*CompletionStage) (shouldTrigger bool, successfulTrigger bool, inputs []*model.CompletionResult)

//triggerAll marks node as succeeded if all are succeeded, or if one has failed
func triggerAll(dependencies []*CompletionStage) (bool, bool, []*model.CompletionResult) {
	var results = make([]*model.CompletionResult, 0)
	for _, s := range dependencies {
		if s.IsFailed() {
			return true, false, []*model.CompletionResult{s.result}
		} else if s.IsSuccessful() {
			results = append(results, s.result)
		}
	}

	if len(results) == len(dependencies) {
		return true, true, results
	}
	return false, false, nil

}

// triggerAny marks a node as succeed if any one is resolved successfully,  or fails with the first error if all are failed
func triggerAny(dependencies []*CompletionStage) (bool, bool, []*model.CompletionResult) {
	var haveUnresolved bool
	var firstFailure *CompletionStage
	if 0 == len(dependencies) {
		// TODO: any({}) -  not clear if we should block here  or error
		return false, false, nil
	}
	for _, s := range dependencies {
		if s.IsResolved() {
			if !s.IsFailed() {
				return true, true, []*model.CompletionResult{s.result}
			}
			firstFailure = s

		} else {
			haveUnresolved = true
		}
	}
	if !haveUnresolved {
		return true, false, []*model.CompletionResult{firstFailure.result}
	}
	return false, false, nil
}

// triggerImmediateSuccess always marks the node as triggered
func triggerImmediateSuccess(stage []*CompletionStage) (bool, bool, []*model.CompletionResult) {
	return true, true, []*model.CompletionResult{}
}

// triggerNever always marks the node as untriggered.
func triggerNever(stage []*CompletionStage) (bool, bool, []*model.CompletionResult) {
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
		Datum:      emptyDatum(),
	})
}

// invokeWithoutArgs triggers invoking the closure with no args
func invokeWithoutArgs(stage *CompletionStage, listener CompletionEventListener, results []*model.CompletionResult) {
	listener.OnExecuteStage(stage, []*model.Datum{})
}

// invokeWithResult triggers invoking the closure with no args
func invokeWithResult(stage *CompletionStage, listener CompletionEventListener, results []*model.CompletionResult) {
	data := make([]*model.Datum, len(results))
	for i, v := range results {
		// TODO: propagate these - this is currently an error in the switch below
		if !v.Successful {
			panic(fmt.Sprintf("Invalid state - tried to invoke stage  %v successfully with failed upstream result %v", stage, v))
		}
		data[i] = v.GetDatum()
	}
	listener.OnExecuteStage(stage, data)
}

// invokeWithError triggers invoking the closure with the error
func invokeWithError(stage *CompletionStage, listener CompletionEventListener, results []*model.CompletionResult) {
	// TODO: This is synonymous with invokeResult as Trigger extracts the error as the result
	data := make([]*model.Datum, len(results))
	for i, v := range results {
		// TODO: propagate these - this is currently an error in the switch below
		if v.Successful {
			panic(fmt.Sprintf("Invalid state - tried to invoke stage %v erroneously with failed upstream result %v", stage, v))
		}
		data[i] = v.GetDatum()
	}
	listener.OnExecuteStage(stage, data)
}

// invokeWithResultOrError triggers invoking the closure with a pair consisting of the  (result,<emtpy>) if the result is successful, or (<empty>,error) if the result is an error
// this can only be used with single-valued result
func invokeWithResultOrError(stage *CompletionStage, listener CompletionEventListener, results []*model.CompletionResult) {
	if len(results) == 1 {
		// TODO: Don't panic
		panic(fmt.Sprintf("Invalid state - tried to invoke single-valued stage %v with incorrect number of inputs %v", stage, results))
	}
	result := results[0]

	var args []*model.Datum
	if result.Successful {
		args = []*model.Datum{result.Datum, emptyDatum()}
	} else {
		args = []*model.Datum{emptyDatum(), result.Datum}
	}

	listener.OnExecuteStage(stage, args)
}

// noop
func completeExternally(stage *CompletionStage, listener CompletionEventListener, results []*model.CompletionResult) {}

func propagateError(stage *CompletionStage, listener CompletionEventListener, results []*model.CompletionResult) {
	if len(results) != 1 {
		// TODO: Don't panic
		panic(fmt.Sprintf("Invalid state - tried to invoke single-valued stage %v with incorrect number of inputs %v", stage, results))
	}
	result := results[0]

	if result.Successful {
		// TODO: Don't panic
		panic(fmt.Sprintf("Invalid state - tried to propagate stage %v with a non-error result %v as an error ", stage, results))
	}
	listener.OnCompleteStage(stage, result)
}

func propagateSuccess(stage *CompletionStage, listener CompletionEventListener, results []*model.CompletionResult) {
	if len(results) != 1 {
		// TODO: Don't panic
		panic(fmt.Sprintf("Invalid state - tried to invoke single-valued stage %v with incorrect number of inputs %v", stage, results))
	}
	result := results[0]

	if !result.Successful {
		// TODO: Don't panic
		panic(fmt.Sprintf("Invalid state - tried to propagate stage %v with an error result %v as an success ", stage, results))
	}
	listener.OnCompleteStage(stage, result)
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
		graph.eventListener.OnCompleteStage(stage, internalErrorResult(model.ErrorDatumType_invalid_stage_response, "stage returned a non-stageref response"))
		return
	}
	refStage := graph.GetStage(result.Datum.GetStageRef().StageRef)
	if nil == refStage {
		graph.eventListener.OnCompleteStage(stage, internalErrorResult(model.ErrorDatumType_invalid_stage_response, "referenced stage not found "))
	}
	log.WithFields(logrus.Fields{"stage_id": stage.ID, "other_id": refStage.ID}).Info("Composing with new stage ")
	graph.eventListener.OnComposeStage(stage, refStage)
}

func parentStageResult(stage *CompletionStage, graph *CompletionGraph, _ *model.CompletionResult) {

	if len(stage.dependencies) != 1 {
		log.WithFields(logrus.Fields{"stage_id": stage.ID}).Warn("Got a parent-result when none was expected")
	} else if !stage.dependencies[0].IsResolved() {
		log.WithFields(logrus.Fields{"stage_id": stage.ID}).Warn("Got a parent-result when parent hadn't completed")
	} else {
		graph.eventListener.OnCompleteStage(stage, stage.dependencies[0].result)
	}
}

func noResultStrategy(_ *CompletionStage, _ *CompletionGraph, _ *model.CompletionResult) {}

type strategy struct {
	TriggerStrategy        TriggerStrategy
	SuccessStrategy        ExecutionStrategy
	FailureStrategy        ExecutionStrategy
	ResultHandlingStrategy ResultHandlingStrategy
}

func getStrategyFromOperation(operation model.CompletionOperation) (strategy, error) {
	switch operation {

	case model.CompletionOperation_acceptEither:
		return strategy{triggerAny, invokeWithResult, propagateError, invocationResult}, nil

	case model.CompletionOperation_applyToEither:
		return strategy{triggerAny, invokeWithResult, propagateError, invocationResult}, nil

	case model.CompletionOperation_thenAcceptBoth:
		return strategy{triggerAll, invokeWithResultOrError, propagateError, invocationResult}, nil

	case model.CompletionOperation_thenApply:
		return strategy{triggerAny, invokeWithResult, propagateError, invocationResult}, nil

	case model.CompletionOperation_thenRun:
		return strategy{triggerAny, invokeWithoutArgs, propagateError, invocationResult}, nil

	case model.CompletionOperation_thenAccept:
		return strategy{triggerAny, invokeWithResult, propagateError, invocationResult}, nil

	case model.CompletionOperation_thenCompose:
		return strategy{triggerAny, invokeWithResult, propagateError, referencedStageResult}, nil

	case model.CompletionOperation_thenCombine:
		return strategy{triggerAll, invokeWithResultOrError, propagateError, invocationResult}, nil

	case model.CompletionOperation_whenComplete:
		return strategy{triggerAny, invokeWithResultOrError, invokeWithResultOrError, parentStageResult}, nil

	case model.CompletionOperation_handle:
		return strategy{triggerAny, invokeWithResultOrError, invokeWithResultOrError, invocationResult}, nil

	case model.CompletionOperation_supply:
		return strategy{triggerImmediateSuccess, invokeWithoutArgs, propagateError, invocationResult}, nil

	case model.CompletionOperation_invokeFunction:
		return strategy{triggerImmediateSuccess, completeExternally, completeExternally, invocationResult}, nil

	case model.CompletionOperation_completedValue:
		return strategy{triggerNever, completeExternally, propagateError, noResultStrategy}, nil

	case model.CompletionOperation_delay:
		return strategy{triggerNever, completeExternally, completeExternally, noResultStrategy}, nil

	case model.CompletionOperation_allOf:
		return strategy{triggerAll, succeedWithEmpty, propagateError, noResultStrategy}, nil

	case model.CompletionOperation_anyOf:
		return strategy{triggerAny, propagateSuccess, propagateError, noResultStrategy}, nil

	case model.CompletionOperation_externalCompletion:
		return strategy{triggerNever, completeExternally, completeExternally, noResultStrategy}, nil

	case model.CompletionOperation_exceptionally:
		return strategy{triggerAny, propagateSuccess, invokeWithError, invocationResult}, nil
	default:
		return strategy{}, fmt.Errorf("Unrecognised operation %s", operation)
	}
}
