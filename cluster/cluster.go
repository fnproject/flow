package cluster

import (
	"fmt"
	"github.com/fnproject/flow/sharding"

	"github.com/sirupsen/logrus"

	"github.com/gin-gonic/gin"
	"github.com/fnproject/flow/model"
	"google.golang.org/grpc"
	"time"
)

var log = logrus.WithField("logger", "cluster")

// Settings holds config for clustering and information about the current node
type Settings struct {
	NodeCount  int
	NodeID     int
	NodePrefix string
	NodePort   int
}

func (s *Settings) nodeAddress(i int) string {
	if i == s.NodeID {
		return fmt.Sprintf("localhost:%d", s.NodePort)
	}
	return fmt.Sprintf("%s:%d", s.nodeName(i), s.NodePort)
}

func (s *Settings) nodeName(index int) string {
	return fmt.Sprintf("%s%d", s.NodePrefix, index)
}

// Manager manages cluster allocation and shard info
type Manager struct {
	settings  *Settings
	extractor sharding.ShardExtractor
	// node -> proxy
	reverseProxies map[string]model.FlowServiceClient
}

// NewManager creates a new cluster manager
func NewManager(settings *Settings, extractor sharding.ShardExtractor) *Manager {
	proxies := make(map[string]model.FlowServiceClient, settings.NodeCount)
	for i := 0; i < settings.NodeCount; i++ {
		nodeName := settings.nodeName(i)
		address:= settings.nodeAddress(i)

		log.WithField("address",address).WithField("node_idx",i).Debug("connecting to shard")
		conn, err := grpc.Dial(address, grpc.WithInsecure(),grpc.WithBackoffConfig(grpc.BackoffConfig{MaxDelay:500 * time.Millisecond}))
		if err != nil {
			panic(err.Error())
		}
		log.WithField("address",address).WithField("node_idx",i).Debug("connected to shard")

		proxies[nodeName] = model.NewFlowServiceClient(conn)

	}
	log.Info(fmt.Sprintf("Created shard proxy with settings: %+v", settings))
	return &Manager{settings: settings, extractor: extractor, reverseProxies: proxies}
}

// LocalShards returns a slice of the shards associated with this clster member
func (m *Manager) LocalShards() (shards []int) {
	for shard := 0; shard < m.extractor.ShardCount(); shard++ {
		nodeIndex := shard % m.settings.NodeCount
		if nodeIndex == m.settings.NodeID {
			shards = append(shards, shard)
		}
	}
	return
}

// GetClient returns a GRPC client for the specified graph
func (m *Manager) GetClient(flowID string) (model.FlowServiceClient, error) {
	idx, err := m.resolveNode(flowID)
	if err != nil {
		return nil, err
	}

	return m.reverseProxies[ m.settings.nodeName(idx)], nil
}

func (m *Manager) resolveNode(flowID string) (int, error) {
	shard, err := m.extractor.ShardID(flowID)
	if err != nil {
		return -1, err
	}
	nodeIndex := shard % m.settings.NodeCount
	log.WithField("flow_id", flowID).WithField("cluster_shard", shard).Debug("Resolved shard")
	return nodeIndex, nil
}

// returns node to forward to, if applicable
func (m *Manager) shouldForward(c *gin.Context) (bool, string) {
	flowID := extractFlowID(c)
	if len(flowID) == 0 {
		return false, ""
	}

	nodeIndex, err := m.resolveNode(flowID)
	nodeName := m.settings.nodeName(nodeIndex)
	log.WithField("flow_id", flowID).WithField("cluster_node", nodeName).Debug("Resolved node")
	if err != nil {
		log.Info(fmt.Sprintf("Failed to resolve node for FlowId %s: %v", flowID, err))
		return false, ""
	}
	if nodeIndex == m.settings.NodeID {
		return false, nodeName
	}
	return true, nodeName
}

func extractFlowID(c *gin.Context) string {
	if c.Request.URL.Path == "/graph" {
		return c.Query("FlowId")
	}
	return c.Param("FlowId")
}


