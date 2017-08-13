package graph

import (
	"github.com/fnproject/completer/model"
	"fmt"
	"github.com/sirupsen/logrus"
)

// TriggerStrategy defines when a stage becomes active, and what the incoming status of the trigger is
// each type may or may not depend on the incoming dependencies of the node
type TriggerStrategy func(deps []*CompletionStage) (bool, TriggerStatus, []*model.CompletionResult)

//triggerAll marks node as succeeded if all are succeeded, or if one has failed
func triggerAll(dependencies []*CompletionStage) (bool, TriggerStatus, []*model.CompletionResult) {
	var results []*model.CompletionResult = make([]*model.CompletionResult, 0)
	for _, s := range dependencies {
		if s.isFailed() {
			return true, TriggerStatus_failed, []*model.CompletionResult{s.result}
		} else if s.isSuccessful() {
			results = append(results, s.result)
		}
	}

	if len(results) == len(dependencies) {
		return true, TriggerStatus_successful, results
	} else {
		return false, TriggerStatus_failed, nil
	}
}

// triggerAny marks a node as succeed if any one is resolved successfully,  or fails with the first error if all are failed
func triggerAny(dependencies []*CompletionStage) (bool, TriggerStatus, []*model.CompletionResult) {
	var haveUnresolved bool
	var firstFailure *CompletionStage
	if 0 == len(dependencies) {
		return false, TriggerStatus_failed, nil
	}
	for _, s := range dependencies {
		if s.isResolved() {
			if !s.isFailed() {
				return true, TriggerStatus_successful, []*model.CompletionResult{s.result}
			} else {
				firstFailure = s
			}
		} else {
			haveUnresolved = true
		}
	}
	if !haveUnresolved {
		return true, TriggerStatus_failed, []*model.CompletionResult{firstFailure.result}
	}
	return false, TriggerStatus_failed, nil
}

// triggerImmediateSuccess always marks the node as triggered
func triggerImmediateSuccess(stage []*CompletionStage) (bool, TriggerStatus, []*model.CompletionResult) {
	return true, TriggerStatus_successful, []*model.CompletionResult{}
}

// triggerNever always marks the node as untriggered.
func triggerNever(stage []*CompletionStage) (bool, TriggerStatus, []*model.CompletionResult) {
	return false, TriggerStatus_failed, []*model.CompletionResult{}
}

// ExecutionStrategy defines how the node should behave when it is triggered
// (optionally calling the attached node closure and how the arguments to the closure are defined)
// different strategies can be applied when the stage completes successfully or exceptionally
type ExecutionStrategy func(stage *CompletionStage, listener CompletionEventListener, results []*model.CompletionResult)

func emptyDatum() *model.Datum {
	return &model.Datum{Val: &model.Datum_Empty{Empty: &model.EmptyDatum{}}}
}

// succeedWithEmpty triggers completion of the stage with an empty success
func succeedWithEmpty(stage *CompletionStage, listener CompletionEventListener, results []*model.CompletionResult) {
	listener.OnCompleteStage(stage, &model.CompletionResult{
		Status: model.ResultStatus_succeeded,
		Datum:  emptyDatum(),
	})
}

// invokeWithoutArgs triggers invoking the closure with no args
func invokeWithoutArgs(stage *CompletionStage, listener CompletionEventListener, results []*model.CompletionResult) {
	listener.OnExecuteStage(stage, []*model.Datum{})
}

// invokeWithoutArgs triggers invoking the closure with no args
func invokeWithResult(stage *CompletionStage, listener CompletionEventListener, results []*model.CompletionResult) {
	data := make([]*model.Datum, len(results))
	for i, v := range results {
		// TODO: propagate these - this is currently an error in the switch below
		if v.GetStatus() != model.ResultStatus_succeeded {
			panic(fmt.Sprintf("Invalid state - tried to invoke stage  %v successfully with failed upstream result %v", stage, v))
		}
		data[i] = v.GetDatum()
	}
	listener.OnExecuteStage(stage, data)
}

// invokeWithoutArgs triggers invoking the closure with the error
func invokeWithError(stage *CompletionStage, listener CompletionEventListener, results []*model.CompletionResult) {
	// TODO: This is synonymous with invokeResult as Trigger extracts the error as the result
	data := make([]*model.Datum, len(results))
	for i, v := range results {
		// TODO: propagate these - this is currently an error in the switch below
		if v.GetStatus() != model.ResultStatus_failed {
			panic(fmt.Sprintf("Invalid state - tried to invoke stage %v erroneously with failed upstream result %v", stage, v))
		}
		data[i] = v.GetDatum()
	}
	listener.OnExecuteStage(stage, data)
}

func invokeWithResultOrError(stage *CompletionStage, listener CompletionEventListener, results []*model.CompletionResult) {
	if len(results) == 1 {
		// TODO: Don't panic
		panic(fmt.Sprintf("Invalid state - tried to invoke single-valued stage %v with incorrect number of inputs %v", stage, results))
	}
	result := results[0]

	var args []*model.Datum
	if result.Status == model.ResultStatus_failed {
		args = []*model.Datum{emptyDatum(), result.Datum}
	} else {
		args = []*model.Datum{result.Datum, emptyDatum()}
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

	if result.Status != model.ResultStatus_failed {
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

	if result.Status != model.ResultStatus_succeeded {
		// TODO: Don't panic
		panic(fmt.Sprintf("Invalid state - tried to propagate stage %v with an error result %v as an success ", stage, results))
	}
	listener.OnCompleteStage(stage, result)
}

// ResultHandlingStrategy defines how the  result value of the stage is derived - and whether or not it actually completes the node
type ResultHandlingStrategy func(*CompletionStage, *CompletionGraph, *model.CompletionResult)

func invocationResult(stage *CompletionStage, graph *CompletionGraph, result *model.CompletionResult) {
	graph.eventProcessor.OnCompleteStage(stage, result)
}

func referencedStageResult(stage *CompletionStage, graph *CompletionGraph, result *model.CompletionResult) {

	if result.Status == model.ResultStatus_failed {
		// Errors fail the normal way
		graph.eventProcessor.OnCompleteStage(stage, result)
		return
	}
	// result must be a stageref
	if nil == result.Datum.GetStageRef() {
		//TODO complete stage with an error
		graph.eventProcessor.OnCompleteStage(stage, internalErrorResult(model.ErrorDatumType_invalid_stage_response, "stage returned a non-stageref response"))
		return
	}
	refStage := graph.GetStage(StageId(result.Datum.GetStageRef().StageRef))
	if nil == refStage {
		graph.eventProcessor.OnCompleteStage(stage, internalErrorResult(model.ErrorDatumType_invalid_stage_response, "referenced stage not found "))
	}
	log.WithFields(logrus.Fields{"stage_id": stage.Id, "other_id": refStage.Id}).Info("Composing with new stage ")
	graph.eventProcessor.OnComposeStage(stage, refStage)
}

func parentStageResult(stage *CompletionStage, graph *CompletionGraph, result *model.CompletionResult) {

	if len(stage.dependencies) != 1 {
		log.WithFields(logrus.Fields{"stage_id": stage.Id}).Warn("Got a parent-result when none was expected")
	} else if !stage.dependencies[0].isResolved() {
		log.WithFields(logrus.Fields{"stage_id": stage.Id}).Warn("Got a parent-result when parent hadn't completed")
	} else {
		graph.eventProcessor.OnCompleteStage(stage, stage.dependencies[0].result)
	}
}

func noResultStrategy(stage *CompletionStage, graph *CompletionGraph, result *model.CompletionResult) {

}

type strategy struct {
	TriggerStrategy        TriggerStrategy
	SuccessStrategy        ExecutionStrategy
	FailureStrategy        ExecutionStrategy
	ResultHandlingStrategy ResultHandlingStrategy
}

func GetStrategyFromOperation(operation model.CompletionOperation) (strategy, error) {

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
		return strategy{}, fmt.Errorf("Unrecognised op¬eration %s", operation)
	}
}
