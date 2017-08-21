package query

import (
	"testing"
	"github.com/stretchr/testify/require"
)

func TestShouldDecodeJsonCommand(t *testing.T) {

	msg := "{\"type\":\"subscribe\",\"graph_id\":\"g1\"}"

	cmd, err := extractCommand([]byte(msg))

	require.NoError(t,err)
	require.Equal(t,&subscribeGraph{GraphID:"g1"}, cmd)
}
