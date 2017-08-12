package graph

import (
	"testing"
	"github.com/fnproject/completer/model"
	"github.com/stretchr/testify/assert"
)

func TestAllOperationsShouldHaveValidStrategies(t *testing.T) {
	for _, op_idx := range model.CompletionOperation_value {
		operation := model.CompletionOperation(op_idx)
		if operation != model.CompletionOperation_unknown_operation {
			_, err := GetStrategyFromOperation(operation)
			assert.NoError(t, err, "no strategy for operation ")
		}
	}
}

func TestUnknownOperationShouldRaiseError(t *testing.T) {
	_, err := GetStrategyFromOperation(model.CompletionOperation_unknown_operation)
	assert.Error(t, err)
}
