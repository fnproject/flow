package query

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestShouldDecodeJsonCommands(t *testing.T) {
	cases := []struct {
		msg string
		res interface{}
	}{{"{\"command\":\"subscribe\",\"flow_id\":\"g1\"}", &subscribeGraph{FlowID: "g1"}},
		{"{\"command\":\"unsubscribe\",\"flow_id\":\"g1\"}", &unSubscribeGraph{FlowID: "g1"}}}

	for _, c := range cases {
		res, err := extractCommand([]byte(c.msg))
		assert.NoError(t, err)
		assert.Equal(t, c.res, res)
	}
}
