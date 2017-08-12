package graph

import (
	"testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/fnproject/completer/model"
)

type MockedListener struct {
	mock.Mock
}

func (mock *MockedListener) OnCompleteStage(stage *CompletionStage, result *model.CompletionResult) {
	mock.Called(stage, result)
}

func (mock *MockedListener) OnCompleteGraph() {
	mock.Called()
}

func (mock *MockedListener) OnExecuteStage(stage *CompletionStage, datum []*model.Datum) {
	mock.Called(stage, datum)
}

func (mock *MockedListener) OnComposeStage(stage *CompletionStage, composedStage *CompletionStage) {
	mock.Called(stage, composedStage)
}

func TestShouldCreateGraph(t *testing.T) {

	graph := New(GraphId("graph"), "function", nil)
	assert.Equal(t, GraphId("graph"), graph.Id)
	assert.Equal(t, "function", graph.FunctionId)

}

func TestShouldTriggerImmediateInvocationOfClosureNodeWithNoDeps(t *testing.T) {
	m := &MockedListener{}

	m.On("OnExecuteStage", mock.AnythingOfType("*graph.CompletionStage"), mock.AnythingOfType("[]*model.Datum")).Return()

	g := New(GraphId("graph"), "function", m)

	g.AddStage(&model.StageAddedEvent{
		StageId:      1,
		Op:           model.CompletionOperation_supply,
		Closure:      &model.BlobDatum{DataString: []byte("foo"), ContentType: "application/octet-stream"},
		Dependencies: []uint32{},
	}, true)

	m.AssertExpectations(t)

	stage := m.Calls[0].Arguments[0].(*CompletionStage)
	assert.Equal(t, stage.Id, StageId(1))
	assert.Equal(t, stage.operation, model.CompletionOperation_supply)

	input := m.Calls[0].Arguments[1].([]*model.Datum)
	assert.Empty(t, input)
}
