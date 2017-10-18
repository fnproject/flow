package proxy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	validGraphID   = "fbff4e35-d3a0-4185-976a-68ef025282f0"
	invalidGraphID = "invalid-id"
)

var (
	defaultSettings = &ClusterSettings{
		NodeCount:  2,
		NodeName:   "node-1",
		NodePrefix: "node-",
		ShardCount: 20,
	}
	defaultProxy = &ClusterProxy{
		settings: defaultSettings,
	}
)

func TestShardIsStable(t *testing.T) {
	shard, e := defaultProxy.extractShard(validGraphID)
	assert.Nil(t, e)
	assert.True(t, shard >= 0 && shard < 20)
	shard, e = defaultProxy.extractShard(validGraphID)
	assert.Nil(t, e)
	assert.True(t, shard >= 0 && shard < 20)
}

func TestShardForInvalidGraphID(t *testing.T) {
	_, e := defaultProxy.extractShard(invalidGraphID)
	assert.Error(t, e)
}

func TestShardMapping(t *testing.T) {
	node, e := defaultProxy.resolveNode(validGraphID)
	assert.Nil(t, e)
	assert.Contains(t, node, defaultSettings.NodePrefix)
}
func TestInvalidShardMapping(t *testing.T) {
	_, e := defaultProxy.resolveNode(invalidGraphID)
	assert.Error(t, e)
}
