package cluster

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/fnproject/completer/sharding"

	"github.com/sirupsen/logrus"

	"github.com/gin-gonic/gin"
)

var log = logrus.WithField("logger", "proxy")

type ClusterSettings struct {
	NodeCount  int
	NodeID     int
	NodePrefix string
}

func (s *ClusterSettings) nodeName(index int) string {
	return fmt.Sprintf("%s%d", s.NodePrefix, index)
}

type ClusterManager struct {
	settings  *ClusterSettings
	extractor sharding.ShardExtractor
	// node -> proxy
	reverseProxies map[string]*httputil.ReverseProxy
}

func NewManager(settings *ClusterSettings, extractor sharding.ShardExtractor) *ClusterManager {
	proxies := make(map[string]*httputil.ReverseProxy, settings.NodeCount)
	for i := 0; i < settings.NodeCount; i++ {
		node := settings.nodeName(i)
		p, err := newReverseProxy(node)
		if err != nil {
			panic("Failed to generate proxy URL " + err.Error())
		}
		proxies[node] = p
	}
	log.Info(fmt.Sprintf("Created shard proxy with settings: %+v", settings))
	return &ClusterManager{settings: settings, extractor: extractor, reverseProxies: proxies}
}

func (m *ClusterManager) LocalShards() (shards []int) {
	for shard := 0; shard < m.extractor.ShardCount(); shard++ {
		nodeIndex := shard % m.settings.NodeCount
		if nodeIndex == m.settings.NodeID {
			shards = append(shards, shard)
		}
	}
	return
}

func newReverseProxy(node string) (*httputil.ReverseProxy, error) {
	// TODO honor ports passed by configuration
	url, err := url.Parse(fmt.Sprintf("http://%s:%d", node, 8080))
	log.Infof("Registering proxy to %v", url)
	if err != nil {
		return nil, err
	}
	return httputil.NewSingleHostReverseProxy(url), nil
}

func (m *ClusterManager) forward(writer http.ResponseWriter, req *http.Request, node string) error {
	proxy, ok := m.reverseProxies[node]
	if !ok {
		return errors.New(fmt.Sprintf("Missing proxy for node %s", node))
	}
	proxy.ServeHTTP(writer, req)
	return nil
}

func (m *ClusterManager) resolveNode(graphID string) (string, error) {
	shard, err := m.extractor.ShardID(graphID)
	if err != nil {
		return "", err
	}
	nodeIndex := shard % m.settings.NodeCount
	log.WithField("graph_id", graphID).WithField("cluster_shard", shard).Info("Resolved shard")
	return m.settings.nodeName(nodeIndex), nil
}

// returns node to forward to, if applicable
func (m *ClusterManager) shouldForward(c *gin.Context) (bool, string) {
	graphID := extractGraphId(c)
	if len(graphID) == 0 {
		return false, ""
	}

	node, err := m.resolveNode(graphID)
	log.WithField("graph_id", graphID).WithField("cluster_node", node).Info("Resolved node")
	if err != nil {
		log.Info(fmt.Sprintf("Failed to resolve node for graphId %s: %v", graphID, err))
		return false, ""
	}
	localNode := fmt.Sprintf("%s%d", m.settings.NodePrefix, m.settings.NodeID)
	if node == localNode {
		return false, node
	}
	return true, node
}

func extractGraphId(c *gin.Context) string {
	return c.Param("graphId")
}

func (m *ClusterManager) ProxyHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		forward, node := m.shouldForward(c)
		if !forward {
			log.Info("Processing request locally")
			c.Next()
			return
		}
		graphID := extractGraphId(c)
		log.WithField("graph_id", graphID).WithField("proxy_node", node).Info("Proxying graph request")
		if err := m.forward(c.Writer, c.Request, node); err != nil {
			// TODO should we retry if this fails? buffer requests while upstream is unavailable?
			log.WithField("graph_id", graphID).WithField("proxy_node", node).Warn("Failed to proxy graph request")
			c.AbortWithError(502, errors.New("Failed to proxy graph request"))
			return
		}
		c.Abort()
	}
}
