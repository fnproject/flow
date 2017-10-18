package proxy

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/gin-gonic/gin"
)

var log = logrus.WithField("logger", "proxy")

type ClusterSettings struct {
	NodeCount  int
	ShardCount int
	NodeName   string
	NodePrefix string
}

func (s *ClusterSettings) nodeName(index int) string {
	return fmt.Sprintf("%s%d", s.NodePrefix, index)
}

type ClusterProxy struct {
	settings *ClusterSettings
	// node -> proxy
	reverseProxies map[string]*httputil.ReverseProxy
}

func NewProxy(settings *ClusterSettings) *ClusterProxy {
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
	return &ClusterProxy{settings: settings, reverseProxies: proxies}
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

func (p *ClusterProxy) extractShard(graphID string) (int, error) {
	if UUID, err := uuid.Parse(graphID); err != nil {
		return 0, err
	} else {
		lowBits := binary.LittleEndian.Uint64(UUID[:8])
		hiBits := binary.LittleEndian.Uint64(UUID[8:])
		hilo := lowBits ^ hiBits
		shard := hilo % uint64(p.settings.ShardCount)
		return int(shard), nil
	}
}

func (p *ClusterProxy) forward(writer http.ResponseWriter, req *http.Request, node string) error {
	proxy, ok := p.reverseProxies[node]
	if !ok {
		return errors.New(fmt.Sprintf("Missing proxy for node %s", node))
	}
	proxy.ServeHTTP(writer, req)
	return nil
}

func (p *ClusterProxy) resolveNode(graphID string) (string, error) {
	shard, err := p.extractShard(graphID)
	if err != nil {
		return "", err
	}
	nodeIndex := shard % p.settings.NodeCount
	log.WithField("graph_id", graphID).WithField("cluster_shard", shard).Info("Resolved shard")
	return p.settings.nodeName(nodeIndex), nil
}

// returns node to forward to, if applicable
func (p *ClusterProxy) shouldForward(c *gin.Context) (bool, string) {
	if p.settings.NodeCount < 2 {
		return false, ""
	}
	graphID := extractGraphId(c)
	if len(graphID) == 0 {
		return false, ""
	}

	node, err := p.resolveNode(graphID)
	log.WithField("graph_id", graphID).WithField("cluster_node", node).Info("Resolved node")
	if err != nil {
		log.Info(fmt.Sprintf("Failed to resolve node for graphId %s: %v", graphID, err))
		return false, ""
	}
	if node == p.settings.NodeName {
		// the assigned node is the local node
		return false, node
	}
	return true, node
}

func extractGraphId(c *gin.Context) string {
	return c.Param("graphId")
}

func (p *ClusterProxy) ProxyHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		forward, node := p.shouldForward(c)
		if !forward {
			log.Info("Processing request locally")
			c.Next()
			return
		}
		graphID := extractGraphId(c)
		log.WithField("graph_id", graphID).WithField("proxy_node", node).Info("Proxying graph request")
		if err := p.forward(c.Writer, c.Request, node); err != nil {
			// TODO should we retry if this fails? buffer requests while upstream is unavailable?
			log.WithField("graph_id", graphID).WithField("proxy_node", node).Warn("Failed to proxy graph request")
			c.AbortWithError(502, errors.New("Failed to proxy graph request"))
		}
		c.Abort()
	}
}
