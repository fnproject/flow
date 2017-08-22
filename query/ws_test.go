package query

import (
	"testing"
	"github.com/stretchr/testify/assert"
)

func TestShouldDecodeJsonCommands(t *testing.T) {
	cases := []struct {
		msg string
		res interface{}
	}{{"{\"command\":\"subscribe\",\"graph_id\":\"g1\"}", &subscribeGraph{GraphID: "g1"}},
		{"{\"command\":\"unsubscribe\",\"graph_id\":\"g1\"}", &unSubscribeGraph{GraphID: "g1"}}}

	for _, c := range cases {
		res, err := extractCommand([]byte(c.msg))
		assert.NoError(t, err)
		assert.Equal(t, c.res, res)
	}
}
