package cluster

import (
	"testing"

	"github.com/fnproject/flow/sharding"

	"github.com/stretchr/testify/assert"
)

const (
	validFlowID   = "fbff4e35-d3a0-4185-976a-68ef025282f0"
	invalidFlowID = "invalid-id"
)

var (
	defaultSettings = &Settings{
		NodeCount:  2,
		NodeID:     1,
		NodePrefix: "node-",
	}
	defaultManager = &Manager{
		settings:  defaultSettings,
		extractor: sharding.NewFixedSizeExtractor(defaultSettings.NodeCount * 10),
	}
)

func TestShardMapping(t *testing.T) {
	node, e := defaultManager.resolveNode(validFlowID)
	assert.Nil(t, e)
	assert.True(t, node >= 0 && node < defaultSettings.NodeCount)
}
func TestInvalidShardMapping(t *testing.T) {
	_, e := defaultManager.resolveNode(invalidFlowID)
	assert.Error(t, e)
}
