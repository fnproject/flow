package graph

import (
	"testing"
	"github.com/fnproject/completer/model"
	"github.com/stretchr/testify/assert"
)

func TestAllKnownOperationsShouldHaveValidStrategies(t *testing.T) {
	for _, opIdx := range model.CompletionOperation_value {
		operation := model.CompletionOperation(opIdx)
		if operation != model.CompletionOperation_unknown_operation {
			s, err := getStrategyFromOperation(operation)
			assert.NoError(t, err, "no strategy for operation ", operation)
			assert.NotNil(t, s.TriggerStrategy, "no trigger strategy for %v", operation)
			assert.NotNil(t, s.SuccessStrategy, "no success strategy for %v", operation)
			assert.NotNil(t, s.FailureStrategy, "no failure strategy for %v", operation)
			assert.NotNil(t, s.ResultHandlingStrategy, "No result strategy for %v", operation)

		}
	}
}

func TestUnknownOperationShouldRaiseError(t *testing.T) {
	_, err := getStrategyFromOperation(model.CompletionOperation_unknown_operation)
	assert.Error(t, err)
}

func TestTriggerAllEmpty(t *testing.T) {
	trigger, status, inputs := triggerAll([]*CompletionStage{})
	assert.True(t, trigger)
	assert.Equal(t, true, status)
	assert.Empty(t, inputs)

}

func TestTriggerAllCompleted(t *testing.T) {
	s1 := completedStage()
	s2 := completedStage()
	trigger, status, inputs := triggerAll([]*CompletionStage{s1,s2})
	assert.True(t, trigger)
	assert.Equal(t, true, status)
	assert.Equal(t, []*model.CompletionResult{s1.result,s2.result},inputs)

}

func TestTriggerAllPartial(t *testing.T) {
	trigger, _, _ := triggerAll([]*CompletionStage{completedStage(),pendingStage()})
	assert.False(t, trigger)
}

func TestTriggerAllPartialOneFailed(t *testing.T) {
	failed := failedStage()
	trigger, res, inputs := triggerAll([]*CompletionStage{completedStage(),pendingStage(), failed})
	assert.True(t, trigger)
	assert.Equal(t, false,res)
	assert.Equal(t,[]*model.CompletionResult{failed.result},inputs)
}



func TestTriggerAnyEmpty(t *testing.T){
	trigger, _, _ := triggerAny([]*CompletionStage{})
	assert.False(t, trigger)

}


func TestTriggerAnyPartial(t *testing.T){
	s1 := completedStage()
	trigger, status, inputs := triggerAny([]*CompletionStage{s1,pendingStage()})
	assert.True(t, trigger)
	assert.Equal(t, true,status)
	assert.Equal(t,[]*model.CompletionResult{s1.result},inputs)

}


func TestTriggerAnyPartialFailure(t *testing.T){
	s1 := failedStage()
	trigger, _, _ := triggerAny([]*CompletionStage{s1,pendingStage()})
	assert.False(t, trigger)
}
func completedStage() *CompletionStage {
	return &CompletionStage{ID: StageID(1), result: successfulResult(emptyDatum())}
}

func TestTriggerAnyFail(t *testing.T){
	s1 := failedStage()
	trigger, status, inputs := triggerAny([]*CompletionStage{s1})
	assert.True(t, trigger)
	assert.Equal(t, false,status)
	assert.Equal(t,[]*model.CompletionResult{s1.result},inputs)

}

func failedStage() *CompletionStage {
	return &CompletionStage{ID: StageID(1), result: failedResult(emptyDatum())}
}

func pendingStage() *CompletionStage {
	return &CompletionStage{ID: StageID(1)}
}
