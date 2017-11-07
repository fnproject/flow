package sharding

import (
	"encoding/binary"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

var log = logrus.WithField("logger", "sharding")

// ShardExtractor maps graph IDs to shards
type ShardExtractor interface {
	ShardID(graphID string) (int, error)
	ShardCount() int
}

type fixedSizeShardExtractor struct {
	shardCount int
}

// NewFixedSizeExtractor creates  shard mapper with a fixed number of shards using modular arithmetic on the UUID 128-bit value
func NewFixedSizeExtractor(shardCount int) ShardExtractor {
	log.Infof("Initialized shard extractor with %d shards", shardCount)
	return &fixedSizeShardExtractor{shardCount: shardCount}
}

func (m *fixedSizeShardExtractor) ShardID(graphID string) (int, error) {
	UUID, err := uuid.Parse(graphID)
	if err != nil {
		return 0, err
	}
	lowBits := binary.BigEndian.Uint64(UUID[:8])
	hiBits := binary.BigEndian.Uint64(UUID[8:])
	hilo := lowBits ^ hiBits
	shard := hilo % uint64(m.shardCount)
	log.Debugf("Got shard %d for graph %s", int(shard), graphID)
	return int(shard), nil

}

func (m *fixedSizeShardExtractor) ShardCount() int {
	return m.shardCount
}
