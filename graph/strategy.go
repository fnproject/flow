package graph

import "github.com/fnproject/completer/model"

type Trigger uint8

const (
	// trigger when all deps are satisfied
	triggerAll = Trigger(iota)
	// trigger when one dep is satisified
	triggerAny = Trigger(iota)
	// trigger now, irrespective of deps
	triggerImmediate = Trigger(iota)
	// don't trigger (something else triggers this)
	triggerNever = Trigger(iota)
)

type ExecutionStrategy uint8

const (
	// Complete with an empty value without executing a closure
	completeWithVoid        = ExecutionStrategy(iota)
	// Invoke the closure without args,
	invokeWithoutArgs       = ExecutionStrategy(iota)
	// Invoke the closure with the single value from the dependencies
	invokeWithResult        = ExecutionStrategy(iota)
	// Invoke the closure with the single error value from the dependencies
	invokeWithError         = ExecutionStrategy(iota)
	// Invoke the closure with a pair of result (or empty) and error (or empty)
	invokeWithResultOrError = ExecutionStrategy(iota)
	// Completes externally - no closure
	completeExternally      = ExecutionStrategy(iota)
	// Propagate error without  invoking the closure
	completeWithError       = ExecutionStrategy(iota)
	// Propagate the success without invoking the closure
	completeWithParent      = ExecutionStrategy(iota)
)

type ResultHandlingStrategy uint8

const (
	// take success or failed value from closure as value, update status respectively
	invocationResult      = ResultHandlingStrategy(iota)
	// take successful result as a new stage to compose into this stage, proparage errors normally
	referencedStageResult = ResultHandlingStrategy(iota)
	// Take the incoming result from dependencies
	parentStageResult     = ResultHandlingStrategy(iota)
	// Propagate an empty value on success, propagate the error on failure q
	noResultStrategy      = ResultHandlingStrategy(iota)
)

type strategy struct {
	Trigger                Trigger
	SuccessStrategy        ExecutionStrategy
	FailureStrategy        ExecutionStrategy
	ResultHandlingStrategy ResultHandlingStrategy
}

func GetStrategyFromOperation(operation model.CompletionOperation) strategy {

	switch operation {

	case model.CompletionOperation_acceptEither:
		return strategy{triggerAny, invokeWithResult, completeWithError, invocationResult}

	case model.CompletionOperation_applyToEither:
		return strategy{triggerAny, invokeWithResult, completeWithError, invocationResult}

	case model.CompletionOperation_thenAcceptBoth:
		return strategy{triggerAll, invokeWithResultOrError, completeWithError, invocationResult}

	case model.CompletionOperation_thenApply:
		return strategy{triggerAny, invokeWithResult, completeWithError, invocationResult}

	case model.CompletionOperation_thenRun:
		return strategy{triggerAny, invokeWithoutArgs, completeWithError, invocationResult}

	case model.CompletionOperation_thenAccept:
		return strategy{triggerAny, invokeWithResult, completeWithError, invocationResult}

	case model.CompletionOperation_thenCompose:
		return strategy{triggerAny, invokeWithResult, completeWithError, referencedStageResult}

	case model.CompletionOperation_thenCombine:
		return strategy{triggerAll, invokeWithResultOrError, completeWithError, invocationResult}

	case model.CompletionOperation_whenComplete:
		return strategy{triggerAny, invokeWithResultOrError, invokeWithResultOrError, parentStageResult}

	case model.CompletionOperation_handle:
		return strategy{triggerAny, invokeWithResultOrError, invokeWithResultOrError, invocationResult}

	case model.CompletionOperation_supply:
		return strategy{triggerImmediate, invokeWithoutArgs, completeWithError, invocationResult}

	case model.CompletionOperation_invokeFunction:
		return strategy{triggerImmediate, completeExternally, completeExternally, invocationResult}

	case model.CompletionOperation_completedValue:
		return strategy{triggerNever, completeExternally, completeWithError, noResultStrategy}

	case model.CompletionOperation_delay:
		return strategy{triggerNever, completeExternally, completeExternally, noResultStrategy}

	case model.CompletionOperation_allOf:
		return strategy{triggerAll, completeWithVoid, completeWithError, noResultStrategy}

	case model.CompletionOperation_anyOf:
		return strategy{triggerAny, completeWithParent, completeWithError, noResultStrategy}

	case model.CompletionOperation_externalCompletion:
		return strategy{triggerNever, completeExternally, completeExternally, noResultStrategy}

	case model.CompletionOperation_exceptionally:
		return strategy{triggerAny, completeWithParent, invokeWithError, invocationResult}
	default:
		panic("Unhandled completion type")
	}
}
