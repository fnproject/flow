package graph

import (
	"testing"
	"github.com/fnproject/completer/model"
	"github.com/stretchr/testify/assert"
)

func TestAllKnownOperationsShouldHaveValidStrategies(t *testing.T) {
	for _, op_idx := range model.CompletionOperation_value {
		operation := model.CompletionOperation(op_idx)
		if operation != model.CompletionOperation_unknown_operation {
			s, err := GetStrategyFromOperation(operation)
			assert.NoError(t, err, "no strategy for operation ",operation)
			assert.NotNil(t,s.TriggerStrategy,"no trigger strategy for %v",operation)
			assert.NotNil(t,s.SuccessStrategy,"no success strategy for %v",operation)
			assert.NotNil(t,s.FailureStrategy,"no failure strategy for %v",operation)
			assert.NotNil(t,s.ResultHandlingStrategy, "No result strategy for %v" ,operation)

		}
	}
}

func TestUnknownOperationShouldRaiseError(t *testing.T) {
	_, err := GetStrategyFromOperation(model.CompletionOperation_unknown_operation)
	assert.Error(t, err)
}
