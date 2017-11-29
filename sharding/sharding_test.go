package sharding

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

const (
	invalidFlowID = "invalid-id"
)

var (
	validFlowID    = uuid.New().String()
	shardExtractor = NewFixedSizeExtractor(10)
)

func TestShardIsStable(t *testing.T) {
	shard, e := shardExtractor.ShardID(validFlowID)
	assert.Nil(t, e)
	assert.True(t, shard >= 0 && shard < 20)
	shard, e = shardExtractor.ShardID(validFlowID)
	assert.Nil(t, e)
	assert.True(t, shard >= 0 && shard < 20)
}

func TestShardForInvalidFlowId(t *testing.T) {
	_, e := shardExtractor.ShardID(invalidFlowID)
	assert.Error(t, e)
}
