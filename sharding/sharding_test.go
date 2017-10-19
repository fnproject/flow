package sharding

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

const (
	invalidGraphID = "invalid-id"
)

var (
	validGraphID   = uuid.New().String()
	shardExtractor = NewFixedSizeExtractor(10)
)

func TestShardIsStable(t *testing.T) {
	shard, e := shardExtractor.ShardID(validGraphID)
	assert.Nil(t, e)
	assert.True(t, shard >= 0 && shard < 20)
	shard, e = shardExtractor.ShardID(validGraphID)
	assert.Nil(t, e)
	assert.True(t, shard >= 0 && shard < 20)
}

func TestShardForInvalidGraphID(t *testing.T) {
	_, e := shardExtractor.ShardID(invalidGraphID)
	assert.Error(t, e)
}
