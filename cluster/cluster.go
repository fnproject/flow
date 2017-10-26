package cluster

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/fnproject/flow/sharding"

	"github.com/sirupsen/logrus"

	"github.com/gin-gonic/gin"
)

var log = logrus.WithField("logger", "cluster")

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

func (m *ClusterManager) resolveNode(graphID string) (int, error) {
	shard, err := m.extractor.ShardID(graphID)
	if err != nil {
		return -1, err
	}
	nodeIndex := shard % m.settings.NodeCount
	log.WithField("graph_id", graphID).WithField("cluster_shard", shard).Info("Resolved shard")
	return nodeIndex, nil
}

// returns node to forward to, if applicable
func (m *ClusterManager) shouldForward(c *gin.Context) (bool, string) {
	graphID := extractGraphId(c)
	if len(graphID) == 0 {
		return false, ""
	}

	nodeIndex, err := m.resolveNode(graphID)
	nodeName := m.settings.nodeName(nodeIndex)
	log.WithField("graph_id", graphID).WithField("cluster_node", nodeName).Info("Resolved node")
	if err != nil {
		log.Info(fmt.Sprintf("Failed to resolve node for graphId %s: %v", graphID, err))
		return false, ""
	}
	if nodeIndex == m.settings.NodeID {
		return false, nodeName
	}
	return true, nodeName
}

func extractGraphId(c *gin.Context) string {
	if c.Request.URL.Path == "/graph" {
		return c.Query("graphId")
	} else {
		return c.Param("graphId")
	}
}

func (m *ClusterManager) ProxyHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		forward, node := m.shouldForward(c)
		if !forward {
			log.Info("Processing request locally")
			return
		}
		graphID := extractGraphId(c)
		log.WithField("graph_id", graphID).
			WithField("proxy_node", node).
			WithField("proxy_url", c.Request.URL.String()).
			Info("Proxying graph request")

		if err := m.forward(c.Writer, c.Request, node); err != nil {
			// TODO should we retry if this fails? buffer requests while upstream is unavailable?
			log.WithField("graph_id", graphID).
				WithField("proxy_node", node).
				WithField("proxy_url", c.Request.URL.String()).
				Warn("Failed to proxy graph request")
			c.AbortWithError(502, errors.New("Failed to proxy graph request"))
			return
		}
		c.Abort()
	}
}
