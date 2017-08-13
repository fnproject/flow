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

	graph := New(GraphID("graph"), "function", nil)
	assert.Equal(t, GraphID("graph"), graph.ID)
	assert.Equal(t, "function", graph.FunctionID)

}

func TestShouldCreateStageIds(t *testing.T) {

	graph := New(GraphID("graph"), "function", nil)
	assert.Equal(t, uint32(0), graph.NextStageID())
	withSimpleStage(graph, false)
	assert.Equal(t, uint32(1), graph.NextStageID())

}

func TestShouldTriggerNewValueOnAdd(t *testing.T) {
	m := &MockedListener{}

	m.On("OnExecuteStage", mock.AnythingOfType("*graph.CompletionStage"), []*model.Datum{}).Return()

	g := New(GraphID("graph"), "function", m)

	s := withSimpleStage(g, true)

	m.AssertExpectations(t)
	assertStagePending(t, s)
}

func TestShouldNotTriggerNewValueOnNonTrigger(t *testing.T) {
	m := &MockedListener{}

	g := New(GraphID("graph"), "function", m)

	s := withSimpleStage(g, false)

	m.AssertExpectations(t)
	assertStagePending(t, s)

}

func TestShouldCompleteValue(t *testing.T) {
	m := &MockedListener{}

	g := New(GraphID("graph"), "function", m)

	m.On("OnExecuteStage", mock.AnythingOfType("*graph.CompletionStage"), []*model.Datum{}).Return()
	s := withSimpleStage(g, true)

	value := blobDatum(blob("text/plain", []byte("hello")))
	result := successfulResult(value)

	g.HandleStageCompleted(&model.StageCompletedEvent{StageId: uint32(s.ID), Result: result}, true)

	m.AssertExpectations(t)
	assertStageCompletedSuccessfullyWith(t, s, result)
}

func TestShouldTriggerOnCompleteSuccess(t *testing.T) {
	m := &MockedListener{}

	g := New(GraphID("graph"), "function", m)

	s1 := withSimpleStage(g, false)
	s2 := withAppendedStage(g, s1, false)

	// No triggers
	m.AssertExpectations(t)

	value := blobDatum(blob("text/plain", []byte("hello")))
	result := successfulResult(value)

	m.On("OnExecuteStage", s2, []*model.Datum{value}).Return()
	g.HandleStageCompleted(&model.StageCompletedEvent{StageId: uint32(s1.ID), Result: result}, true)
	m.AssertExpectations(t)

}

func TestShouldTriggerOnWhenDependentsAreComplete(t *testing.T) {
	m := &MockedListener{}

	g := New(GraphID("graph"), "function", m)

	s1 := withSimpleStage(g, false)
	value := blobDatum(blob("text/plain", []byte("hello")))

	result := successfulResult(value)
	s1.complete(result)

	m.On("OnExecuteStage", mock.AnythingOfType("*graph.CompletionStage"), []*model.Datum{value}).Return()
	withAppendedStage(g, s1, true)

	// No triggers
	m.AssertExpectations(t)

}

func TestShouldPropagateFailureToSecondStage(t *testing.T) {
	m := &MockedListener{}

	g := New(GraphID("graph"), "function", m)

	s1 := withSimpleStage(g, false)
	s2 := withAppendedStage(g, s1, false)

	// No triggers
	m.AssertExpectations(t)

	value := blobDatum(blob("text/plain", []byte("hello")))
	result := failedResult(value)

	m.On("OnCompleteStage", s2, result).Return()
	g.HandleStageCompleted(&model.StageCompletedEvent{StageId: uint32(s1.ID), Result: result}, true)
	m.AssertExpectations(t)

}

func TestShouldNotTriggerOnSubsequentCompletion(t *testing.T) {
	m := &MockedListener{}

	g := New(GraphID("graph"), "function", m)

	s1 := withSimpleStage(g, false)
	withAppendedStage(g, s1, false)

	// No triggers
	m.AssertExpectations(t)

	value := blobDatum(blob("text/plain", []byte("hello")))
	result := successfulResult(value)

	g.HandleStageCompleted(&model.StageCompletedEvent{StageId: uint32(s1.ID), Result: result}, false)
	m.AssertExpectations(t)

}

//   r:= supply(()->1) // s1
// 	    .thenCompose(()->supply(()->2) // s3) //  s2
func TestShouldTriggerCompose(t *testing.T) {
	m := &MockedListener{}

	g := New(GraphID("graph"), "function", m)
	initial := blobDatum(blob("text/plain", []byte("hello")))
	result := successfulResult(initial)
	s1 := withSimpleStage(g, false)
	s1.complete(result)
	m.On("OnExecuteStage", mock.AnythingOfType("*graph.CompletionStage"), []*model.Datum{initial}).Return()
	s2 := withComposeStage(g, s1, true)
	m.AssertExpectations(t)

	s3 := withSimpleStage(g, false)

	// complete S2 with a ref to s3
	m.On("OnComposeStage", s2, s3).Return()
	g.HandleStageComposed(&model.StageComposedEvent{StageId: uint32(s2.ID), ComposedStageId: uint32(s3.ID)})
	g.HandleInvokeComplete(s2.ID, successfulResult(stageRefDatum(uint32(s3.ID))))
	assertStagePending(t, s2)

	result2 := successfulResult(blobDatum(blob("text/plain", []byte("hello again"))))
	// s2 should now  be completed with s2's result
	m.On("OnCompleteStage", s2, result2).Return()

	g.HandleStageCompleted(&model.StageCompletedEvent{StageId: uint32(s3.ID), Result: result2}, true)

	// No triggers
	m.AssertExpectations(t)

}

func TestShouldFailNodeWhenPartiallyRecovered(t *testing.T) {
	m := &MockedListener{}

	g := New(GraphID("graph"), "function", m)
	s := withSimpleStage(g, false)
	m.On("OnCompleteStage", s, stageRecoveryFailedResult).Return()

	g.Recover()
	m.AssertExpectations(t)

}

func TestShouldGetAllStages(t *testing.T){
	m := &MockedListener{}

	g := New(GraphID("graph"), "function", m)

	assert.Empty(t,g.GetAllStages())
	s:= withSimpleStage(g,false)
	assert.Equal(t,[]*CompletionStage{s},g.GetAllStages())
}



func TestShouldRejectUnknownStage(t *testing.T){
	m := &MockedListener{}

	g := New(GraphID("graph"), "function", m)
	event := &model.StageAddedEvent{
		StageId:      g.NextStageID(),
		Op:           model.CompletionOperation_unknown_operation,
		Closure:      &model.BlobDatum{DataString: []byte("foo"), ContentType: "application/octet-stream"},
		Dependencies: []uint32{},
	}

	assert.Error(t, g.HandleStageAdded(event, false))

}

func TestShouldRejectDuplicateStage(t *testing.T){
	m := &MockedListener{}

	g := New(GraphID("graph"), "function", m)
	s:= withSimpleStage(g,false)
	event := &model.StageAddedEvent{
		StageId:      uint32(s.ID),
		Op:           model.CompletionOperation_thenApply,
		Closure:      &model.BlobDatum{DataString: []byte("foo"), ContentType: "application/octet-stream"},
		Dependencies: []uint32{},
	}


	assert.Error(t, g.HandleStageAdded(event, false))
}



func TestShouldCompleteEmptyGraph(t *testing.T){
	m := &MockedListener{}

	g := New(GraphID("graph"), "function", m)
	m.On("OnCompleteGraph").Return()
	g.HandleCommitted()
	m.AssertExpectations(t)
}


func TestShouldNotCompletePendingGraph(t *testing.T){
	m := &MockedListener{}

	g := New(GraphID("graph"), "function", m)
	withSimpleStage(g,false)
	assert.False(t,g.IsCompleted())
	g.HandleCommitted()
	m.AssertExpectations(t)
}



func TestShouldPreventAddingStageToCompletedGraph(t *testing.T){
	m := &MockedListener{}

	g := New(GraphID("graph"), "function", m)
	m.On("OnCompleteGraph").Return()
	g.HandleCommitted()
	g.HandleCompleted()

	event := &model.StageAddedEvent{
		StageId:      g.NextStageID(),
		Op:           model.CompletionOperation_supply,
		Closure:      &model.BlobDatum{DataString: []byte("foo"), ContentType: "application/octet-stream"},
		Dependencies: []uint32{},
	}

	err:= g.HandleStageAdded(event, false)
	assert.Error(t,err)
	m.AssertExpectations(t)
}


func withSimpleStage(g *CompletionGraph, trigger bool) *CompletionStage {
	event := &model.StageAddedEvent{
		StageId:      g.NextStageID(),
		Op:           model.CompletionOperation_supply,
		Closure:      &model.BlobDatum{DataString: []byte("foo"), ContentType: "application/octet-stream"},
		Dependencies: []uint32{},
	}

	g.HandleStageAdded(event, trigger)
	return g.GetStage(StageID(event.StageId))
}

func withAppendedStage(g *CompletionGraph, s *CompletionStage, trigger bool) *CompletionStage {
	event := &model.StageAddedEvent{
		StageId:      g.NextStageID(),
		Op:           model.CompletionOperation_thenApply,
		Closure:      &model.BlobDatum{DataString: []byte("foo"), ContentType: "application/octet-stream"},
		Dependencies: []uint32{uint32(s.ID)},
	}

	g.HandleStageAdded(event, trigger)
	return g.GetStage(StageID(event.StageId))
}

func withComposeStage(g *CompletionGraph, s *CompletionStage, trigger bool) *CompletionStage {
	event := &model.StageAddedEvent{
		StageId:      g.NextStageID(),
		Op:           model.CompletionOperation_thenCompose,
		Closure:      &model.BlobDatum{DataString: []byte("foo"), ContentType: "application/octet-stream"},
		Dependencies: []uint32{uint32(s.ID)},
	}

	g.HandleStageAdded(event, trigger)
	return g.GetStage(StageID(event.StageId))
}

func assertStagePending(t *testing.T, s *CompletionStage) {
	assert.False(t, s.IsResolved())
	assert.False(t, s.IsFailed())
	assert.False(t, s.IsSuccessful())
}
func assertStageCompletedSuccessfullyWith(t *testing.T, s *CompletionStage, result *model.CompletionResult) {
	assert.Equal(t, result, s.result)
	assert.True(t, s.IsSuccessful())
	assert.True(t, s.IsResolved())
}
