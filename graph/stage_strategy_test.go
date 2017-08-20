package graph

import (
	"testing"

	"github.com/fnproject/completer/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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
	trigger, status, inputs := triggerAll([]*CompletionStage{s1, s2})
	assert.True(t, trigger)
	assert.Equal(t, true, status)
	assert.Equal(t, []*model.CompletionResult{s1.result, s2.result}, inputs)

}

func TestTriggerAllPartial(t *testing.T) {
	trigger, _, _ := triggerAll([]*CompletionStage{completedStage(), pendingStage()})
	assert.False(t, trigger)
}

func TestTriggerAllPartialOneFailed(t *testing.T) {
	failed := failedStage()
	trigger, res, inputs := triggerAll([]*CompletionStage{completedStage(), pendingStage(), failed})
	assert.True(t, trigger)
	assert.Equal(t, false, res)
	assert.Equal(t, []*model.CompletionResult{failed.result}, inputs)
}

func TestTriggerAnyEmpty(t *testing.T) {
	trigger, _, _ := triggerAny([]*CompletionStage{})
	assert.False(t, trigger)

}

func TestTriggerAnyPartial(t *testing.T) {
	s1 := completedStage()
	trigger, status, inputs := triggerAny([]*CompletionStage{s1, pendingStage()})
	assert.True(t, trigger)
	assert.Equal(t, true, status)
	assert.Equal(t, []*model.CompletionResult{s1.result}, inputs)

}

func TestTriggerAnyPartialFailure(t *testing.T) {
	s1 := failedStage()
	trigger, _, _ := triggerAny([]*CompletionStage{s1, pendingStage()})
	assert.False(t, trigger)
}

func TestTriggerAnyCompleteFailure(t *testing.T) {
	trigger, result, inputs := triggerAny([]*CompletionStage{failedStage(), failedStage()})
	assert.True(t, trigger)
	assert.False(t, result)
	assert.Equal(t, []*model.CompletionResult{model.NewFailedResult(model.NewEmptyDatum())}, inputs)
}

func TestTriggerNever(t *testing.T) {
	trigger, _, _ := triggerNever([]*CompletionStage{completedStage()})
	assert.False(t, trigger)
}

func TestSucceedWithEmpty(t *testing.T) {
	m := &MockedListener{}
	s := pendingStage()
	m.On("OnCompleteStage", s, model.NewSuccessfulResult(model.NewEmptyDatum()))

	succeedWithEmpty(s, m, []*model.CompletionResult{})
	m.AssertExpectations(t)
}

func TestInvokeWithResult(t *testing.T) {
	cases := [][]*model.CompletionResult{
		{},
		{model.NewSuccessfulResult(model.NewEmptyDatum())},
		{model.NewSuccessfulResult(model.NewEmptyDatum()), model.NewFailedResult(aBlobDatum())},
	}

	for _, c := range cases {
		m := &MockedListener{}
		s := pendingStage()
		args := make([]*model.Datum, len(c))
		for i, v := range c {
			args[i] = v.Datum
		}

		m.On("OnExecuteStage", s, args)

		invokeWithResult(s, m, c)
		m.AssertExpectations(t)
	}
}

func TestInvokeWithResultOrError(t *testing.T) {

	type resultCase struct {
		input []*model.CompletionResult
		data  []*model.Datum
	}

	cases := []resultCase{
		{input: []*model.CompletionResult{model.NewSuccessfulResult(aBlobDatum())}, data: []*model.Datum{aBlobDatum(), model.NewEmptyDatum()}},
		{input: []*model.CompletionResult{model.NewFailedResult(aBlobDatum())}, data: []*model.Datum{model.NewEmptyDatum(), aBlobDatum()}},
	}

	for _, c := range cases {
		m := &MockedListener{}
		s := pendingStage()

		m.On("OnExecuteStage", s, c.data)
		invokeWithResultOrError(s, m, c.input)
		m.AssertExpectations(t)
	}
}

func TestPropagateResult(t *testing.T) {
	m := &MockedListener{}
	s := pendingStage()
	r := model.NewSuccessfulResult(aBlobDatum())

	m.On("OnCompleteStage", s, r)
	propagateResult(s, m, []*model.CompletionResult{r})
	m.AssertExpectations(t)
}

func TestInvocationResult(t *testing.T) {
	m := &MockedListener{}
	g := New("graph", "fn", m)
	s := pendingStage()

	r := model.NewSuccessfulResult(aBlobDatum())
	m.On("OnCompleteStage", s, r)

	invocationResult(s, g, r)

	m.AssertExpectations(t)

}

func TestReferencedStageResultPropagatesError(t *testing.T) {
	m := &MockedListener{}
	g := New("graph", "fn", m)
	s := pendingStage()

	r := model.NewFailedResult(aBlobDatum())
	m.On("OnCompleteStage", s, r)

	referencedStageResult(s, g, r)

	m.AssertExpectations(t)

}

func TestReferencedStageResultFailsWithInvalidDatum(t *testing.T) {
	m := &MockedListener{}
	g := New("graph", "fn", m)
	s := pendingStage()

	m.On("OnCompleteStage", s, mock.Anything)

	referencedStageResult(s, g, model.NewSuccessfulResult(aBlobDatum()))

	m.AssertExpectations(t)
	err := m.Calls[0].Arguments[1].(*model.CompletionResult)

	assert.NotNil(t, err.GetDatum().GetError())
	assert.Equal(t, model.ErrorDatumType_invalid_stage_response, err.GetDatum().GetError().Type)

}

func TestReferencedStageResultFailsWithUnknownStage(t *testing.T) {
	m := &MockedListener{}
	g := New("graph", "fn", m)
	s := pendingStage()

	m.On("OnCompleteStage", s, mock.Anything)

	referencedStageResult(s, g, model.NewSuccessfulResult(model.NewStageRefDatum("Some_stage")))

	m.AssertExpectations(t)
	err := m.Calls[0].Arguments[1].(*model.CompletionResult)

	assert.NotNil(t, err.GetDatum().GetError())
	assert.Equal(t, model.ErrorDatumType_invalid_stage_response, err.GetDatum().GetError().Type)

}




func TestReferencedStageResultComposesStage(t *testing.T) {
	m := &MockedListener{}

	g := New("graph", "fn", m)
	composed := pendingStage()
	composed.ID = "composed"
	g.stages[composed.ID]= composed

	s := pendingStage()

	m.On("OnComposeStage",s,composed )

	referencedStageResult(s, g, model.NewSuccessfulResult(model.NewStageRefDatum(composed.ID)))

	m.AssertExpectations(t)

}

func TestParentStageResult (t *testing.T) {
	m := &MockedListener{}

	g := New("graph", "fn", m)
	parent := pendingStage()
	parent.ID ="parent"
	r:= model.NewSuccessfulResult(aBlobDatum())
	parent.result  = r

	s := pendingStage()
	s.dependencies = []*CompletionStage{parent}

	m.On("OnCompleteStage",s, r)

	parentStageResult(s, g, model.NewSuccessfulResult(model.NewEmptyDatum()))

	m.AssertExpectations(t)

}

func aBlobDatum() *model.Datum {
	return model.NewBlobDatum(&model.BlobDatum{BlobId:"blob_id",ContentType:"text/play",Length:122})
}

func completedStage() *CompletionStage {
	return &CompletionStage{ID: "1", result: model.NewSuccessfulResult(model.NewEmptyDatum())}
}

func TestTriggerAnyFail(t *testing.T) {
	s1 := failedStage()
	trigger, status, inputs := triggerAny([]*CompletionStage{s1})
	assert.True(t, trigger)
	assert.Equal(t, false, status)
	assert.Equal(t, []*model.CompletionResult{s1.result}, inputs)

}

func failedStage() *CompletionStage {
	return &CompletionStage{ID: "1", result: model.NewFailedResult(model.NewEmptyDatum())}
}

func pendingStage() *CompletionStage {
	return &CompletionStage{ID: "1"}
}
