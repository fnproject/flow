package server

import (
	"errors"
	"net/http"
	"time"

	"net/url"

	"github.com/fnproject/flow/actor"
	"github.com/fnproject/flow/cluster"
	"github.com/fnproject/flow/model"
	"github.com/fnproject/flow/query"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
)

const (
	maxDelayStageDelay = 3600 * 1000 * 24
	maxRequestTimeout  = 1 * time.Hour
	minRequestTimeout  = 1 * time.Second

	paramGraphID   = "graphId"
	paramStageID   = "stageId"
	paramOperation = "operation"

	queryParamGraphID = "graphId"
)

var log = logrus.WithField("logger", "server")

// Server is the flow service root
type GrpcServer struct {
	backend        model.FlowServiceClient
	apiURL         *url.URL
	listen         string
	requestTimeout time.Duration
	promHandler    http.Handler
}


// New creates a new server - params are injected dependencies
func New(clusterManager *cluster.Manager, manager actor.GraphManager, listenAddress string, maxRequestTimeout time.Duration, zipkinURL string) (*Server, error) {

	setTracer(listenAddress, zipkinURL)

	s := &Server{
		GraphManager:   manager,
		Engine:         newEngine(clusterManager),
		listen:         listenAddress,
		requestTimeout: maxRequestTimeout,
		promHandler:    promhttp.Handler(),
	}

	s.Engine.GET("/ping", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	s.Engine.GET("/metrics", s.handlePrometheusMetrics)

	createGraphAPI(s, manager)

	return s, nil
}

// Run starts the server
func (s *Server) Run() {
	log.WithField("listen_url", s.listen).Infof("Starting Completer server (timeout %s) ", s.requestTimeout)

	s.Engine.Run(s.listen)
}

// context handler that intercepts graph create requests, injecting a UUID parameter prior
// to forwarding to the appropriate node in the cluster
func graphCreateInterceptor(c *gin.Context) {
	// TODO need to prevent clients from defining new graph IDs
	if c.Request.URL.Path == "/graph" && len(c.Query(queryParamGraphID)) == 0 {
		UUID, err := uuid.NewRandom()
		if err != nil {
			c.AbortWithError(500, errors.New("failed to generate UUID for new graph"))
			return
		}
		graphID := UUID.String()
		log.Infof("Generated new graph ID %s", graphID)

		// set the graphId query param in the original request prior to proxying
		values := c.Request.URL.Query()
		values.Add(queryParamGraphID, graphID)
		c.Request.URL.RawQuery = values.Encode()
	}
}

func (s *Server) handleAddStage(c *gin.Context) {

}

func createGraphAPI(s *Server, manager actor.GraphManager) {
	s.Engine.GET("/wss", func(c *gin.Context) {
		query.WSSHandler(manager, c.Writer, c.Request)
	})
	graph := s.Engine.Group("/graph")
	{
		graph.POST("/create", s.handleCreateGraph)
		graph.POST("/getGraph", s.handleGraphState)
		graph.POST("/getStage", s.handleGetGraphStage)
		graph.POST("/addStage", s.handleAddStage)
		graph.POST("/completeStage", s.handleCompleteStage)
		graph.POST("/addInvoke", s.handleInvokeFunction)
		graph.POST("/addCompletedValue", s.handleCompletedValue)
		graph.POST("/addDelay", s.handleDelay)
		graph.POST("/commit", s.handleCommit)

	}
}
