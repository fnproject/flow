package graph

import (
	"testing"
	assert "github.com/stretchr/testify/assert"
	mock "github.com/stretchr/testify/mock"
)

type MockedListener struct {
	mock.Mock
}




func TestShouldCreateGraph(t *testing.T) {


	graph := New(GraphId("graph"), "function", nil)
	assert.Equal(t,GraphId("graph"),graph.Id)
	assert.Equal(t,"fucntion",graph.Id)

}
