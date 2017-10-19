package cluster

import (
	"testing"

	"github.com/fnproject/completer/sharding"

	"github.com/stretchr/testify/assert"
)

const (
	validGraphID   = "fbff4e35-d3a0-4185-976a-68ef025282f0"
	invalidGraphID = "invalid-id"
)

var (
	defaultSettings = &ClusterSettings{
		NodeCount:  2,
		NodeID:     1,
		NodePrefix: "node-",
	}
	defaultManager = &ClusterManager{
		settings:  defaultSettings,
		extractor: sharding.NewFixedSizeExtractor(defaultSettings.NodeCount * 10),
	}
)

func TestShardMapping(t *testing.T) {
	node, e := defaultManager.resolveNode(validGraphID)
	assert.Nil(t, e)
	assert.Contains(t, node, defaultSettings.NodePrefix)
}
func TestInvalidShardMapping(t *testing.T) {
	_, e := defaultManager.resolveNode(invalidGraphID)
	assert.Error(t, e)
}
