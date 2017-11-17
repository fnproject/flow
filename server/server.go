package server

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"net/url"

	protoactor "github.com/AsynkronIT/protoactor-go/actor"
	"github.com/fnproject/flow/actor"
	"github.com/fnproject/flow/cluster"
	"github.com/fnproject/flow/model"
	"github.com/fnproject/flow/protocol"
	"github.com/fnproject/flow/query"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/opentracing/opentracing-go"
	"github.com/openzipkin/zipkin-go-opentracing"
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

	queryParamGraphID    = "graphId"
	queryParamFunctionID = "functionId"
)

var log = logrus.WithField("logger", "server")

func (s *Server) completeWithResult(graphID string, stageID string, req *http.Request) (*model.CompleteStageExternallyResponse, error) {

	result, err := protocol.CompletionResultFromRequest( req)
	if err != nil {
		return nil, err
	}

	request := model.CompleteStageExternallyRequest{
		GraphId: graphID,
		StageId: stageID,
		Result:  result,
	}

	response, err := s.GraphManager.CompleteStageExternally(&request, s.requestTimeout)

	return response, err
}

func (s *Server) handleStageOperation(c *gin.Context) {
	graphID := c.Param(paramGraphID)
	if !validGraphID(graphID) {
		renderError(ErrInvalidGraphID, c)
		return
	}
	stageID := c.Param(paramStageID)
	if !validStageID(graphID) {
		renderError(ErrInvalidStageID, c)
		return
	}
	operation := c.Param(paramOperation)

	switch operation {

	case "complete":
		response, err := s.completeWithResult(graphID, stageID, c.Request)
		if err != nil {
			renderError(err, c)
			return
		}
		c.Header(protocol.HeaderStageRef, response.StageId)
		if response.Successful {
			c.Status(http.StatusOK)
		} else {
			c.String(http.StatusConflict, "Stage is already completed")
		}

	default:
		other := c.Query("other")
		cids := []string{stageID}
		if other != "" {
			if !validStageID(other) {
				renderError(ErrInvalidDepStageID, c)
				return
			}
			cids = append(cids, other)
		}

		completionOperation, found := model.CompletionOperation_value[operation]
		if !found {
			renderError(ErrUnrecognisedCompletionOperation, c)
			return

		}

		blob, err := protocol.BlobFromRequest(c.Request)
		if err != nil {
			renderError(err, c)
			return
		}

		codeLoc := c.GetHeader(protocol.HeaderCodeLocation)

		// TODO: enforce valid content type
		// TODO: generic error handling
		request := &model.AddChainedStageRequest{
			GraphId:      graphID,
			Deps:         cids,
			Operation:    model.CompletionOperation(completionOperation),
			Closure:      blob,
			CodeLocation: codeLoc,
			CallerId:     c.GetHeader(protocol.HeaderCallerRef),
		}
		response, err := s.addStage(request)

		if err != nil {
			renderError(err, c)
			return
		}

		c.Header(protocol.HeaderStageRef, response.StageId)
		c.Status(http.StatusOK)
	}
}

func renderError(err error, c *gin.Context) {
	if gin.Mode() == gin.DebugMode {
		log.WithError(err).Debug("Error occurred in request")
	}
	switch e := err.(type) {

	case model.ValidationError, *protocol.BadProtoMessage:
		c.Data(http.StatusBadRequest, "text/plain", []byte(e.Error()))
	case *Error:
		{
			c.Data(e.HTTPStatus, "text/plain", []byte(e.Message))
		}
	default:
		log.WithError(err).Error("Internal server error")
		c.Status(http.StatusInternalServerError)
	}
}

func (s *Server) handleCreateGraph(c *gin.Context) {
	log.Info("Creating graph")
	functionID := c.Query(queryParamFunctionID)
	graphID := c.Query(queryParamGraphID)

	if !validFunctionID(functionID, false) {
		log.WithField("function_id", functionID).Info("Invalid function iD ")
		renderError(ErrInvalidFunctionID, c)
		return
	}
	if !validGraphID(graphID) {
		renderError(ErrInvalidGraphID, c)
		return
	}

	req := &model.CreateGraphRequest{FunctionId: functionID, GraphId: graphID}
	result, err := s.GraphManager.CreateGraph(req, s.requestTimeout)
	if err != nil {
		renderError(err, c)
		return
	}
	c.Header(protocol.HeaderFlowID, result.GraphId)
	c.Status(http.StatusOK)
}

func (s *Server) handleGraphState(c *gin.Context) {

	graphID := c.Param(paramGraphID)
	if !validGraphID(graphID) {
		renderError(ErrInvalidGraphID, c)
		return
	}

	request := &model.GetGraphStateRequest{GraphId: graphID}
	resp, err := s.GraphManager.GetGraphState(request, s.requestTimeout)

	if err != nil {
		renderError(err, c)
		return
	}
	c.JSON(http.StatusOK, resp)
}


func (s *Server) handleGetGraphStage(c *gin.Context) {
	graphID := c.Param(paramGraphID)
	stageID := c.Param(paramStageID)

	timeout := maxRequestTimeout

	if timeoutMs := c.Query("timeoutMs"); timeoutMs != "" {
		userTimeout, err := time.ParseDuration(timeoutMs + "ms")
		if err != nil {
			renderError(ErrInvalidGetTimeout, c)
			return
		}
		userTimeoutInt := int64(userTimeout)
		if userTimeoutInt == 0 {
			// block "indefinitely"
			timeout = maxRequestTimeout
		} else if userTimeoutInt < int64(minRequestTimeout) {
			// wait at least the minimum request timeout
			timeout = minRequestTimeout
		} else if userTimeoutInt < int64(maxRequestTimeout) {
			timeout = userTimeout
		}
	}

	if !validGraphID(graphID) {
		renderError(ErrInvalidGraphID, c)
		return
	}
	if !validStageID(stageID) {
		renderError(ErrInvalidStageID, c)
		return
	}

	request := model.GetStageResultRequest{
		GraphId: graphID,
		StageId: stageID,
	}

	response, err := s.GraphManager.GetStageResult(&request, timeout)

	if err == protoactor.ErrTimeout {
		c.Data(http.StatusRequestTimeout, "text/plain", []byte("stage not completed"))
		return
	}

	if err != nil {
		renderError(err, c)
		return
	}

	result := response.GetResult()
	err = protocol.WriteResult(protocol.NewResponseWriterTarget(c.Writer),result)

	if err !=nil {
		log.WithError(err).Warn("Failed to write response to client ")
	}

}

func (s *Server) handleExternalCompletion(c *gin.Context) {
	graphID := c.Param(paramGraphID)
	if !validGraphID(graphID) {
		renderError(ErrInvalidGraphID, c)
		return
	}
	request := &model.AddExternalCompletionStageRequest{
		GraphId:      graphID,
		CodeLocation: c.GetHeader(protocol.HeaderCodeLocation),
		CallerId:     c.GetHeader(protocol.HeaderCallerRef),
	}

	response, err := s.addStage(request)

	if err != nil {
		renderError(err, c)
		return
	}
	c.Header(protocol.HeaderStageRef, response.StageId)
	c.Status(http.StatusOK)
}

func (s *Server) allOrAnyOf(c *gin.Context, op model.CompletionOperation) {
	cidList := c.Query("cids")
	graphID := c.Param(paramGraphID)
	if !validGraphID(graphID) {
		renderError(ErrInvalidGraphID, c)
		return
	}
	cids := strings.Split(cidList, ",")

	for _, stageID := range cids {
		if !validStageID(stageID) {
			renderError(ErrInvalidDepStageID, c)
			return
		}
	}

	request := &model.AddChainedStageRequest{
		GraphId:      graphID,
		Operation:    op,
		Closure:      nil,
		Deps:         cids,
		CodeLocation: c.GetHeader(protocol.HeaderCodeLocation),
		CallerId:     c.GetHeader(protocol.HeaderCallerRef),
	}

	response, err := s.addStage(request)

	// TODO: Actually some errors should be user errors here (e.g. AnyOf with zero dependencies)
	if err != nil {
		renderError(err, c)
		return
	}
	c.Header(protocol.HeaderStageRef, response.StageId)
	c.Status(http.StatusOK)
}

func (s *Server) handleAllOf(c *gin.Context) {
	s.allOrAnyOf(c, model.CompletionOperation_allOf)
}

func (s *Server) handleAnyOf(c *gin.Context) {
	s.allOrAnyOf(c, model.CompletionOperation_anyOf)
}

func (s *Server) handleSupply(c *gin.Context) {
	graphID := c.Param(paramGraphID)
	if !validGraphID(graphID) {
		renderError(ErrInvalidGraphID, c)
		return
	}

	blob, err := protocol.BlobFromRequest(c.Request)
	if err != nil {
		renderError(err, c)
		return
	}

	request := &model.AddChainedStageRequest{
		GraphId:      graphID,
		Operation:    model.CompletionOperation_supply,
		Closure:      blob,
		Deps:         []string{},
		CodeLocation: c.GetHeader(protocol.HeaderCodeLocation),
		CallerId:     c.GetHeader(protocol.HeaderCallerRef),
	}

	response, err := s.addStage(request)
	if err != nil {
		renderError(err, c)
		return
	}
	c.Header(protocol.HeaderStageRef, response.StageId)
	c.Status(http.StatusOK)
}

func (s *Server) handleCompletedValue(c *gin.Context) {
	graphID := c.Param(paramGraphID)
	if !validGraphID(graphID) {
		renderError(ErrInvalidGraphID, c)
		return
	}

	result, err := protocol.CompletionResultFromRequest( c.Request)
	if err != nil {
		renderError(err, c)
		return
	}

	request := &model.AddCompletedValueStageRequest{
		GraphId:      graphID,
		Result:       result,
		CodeLocation: c.GetHeader(protocol.HeaderCodeLocation),
		CallerId:     c.GetHeader(protocol.HeaderCallerRef),
	}

	response, err := s.addStage(request)
	if err != nil {
		renderError(err, c)
		return
	}
	c.Header(protocol.HeaderStageRef, response.StageId)
	c.Status(http.StatusOK)
}

func (s *Server) addStage(request model.AddStageCommand) (*model.AddStageResponse, error) {
	return s.GraphManager.AddStage(request, s.requestTimeout)
}

func (s *Server) handleCommit(c *gin.Context) {
	graphID := c.Param(paramGraphID)
	request := model.CommitGraphRequest{GraphId: graphID}

	response, err := s.GraphManager.Commit(&request, s.requestTimeout)
	if err != nil {
		renderError(err, c)
		return
	}

	c.Header(protocol.HeaderFlowID, response.GraphId)
	c.Status(http.StatusOK)
}

func (s *Server) handleDelay(c *gin.Context) {
	graphID := c.Param(paramGraphID)
	if !validGraphID(graphID) {
		renderError(ErrInvalidGraphID, c)
		return
	}
	delayMs := c.Query("delayMs")
	if delayMs == "" {
		renderError(ErrMissingOrInvalidDelay, c)
		return
	}

	delay, err := strconv.ParseInt(delayMs, 10, 64)
	if err != nil || delay < 0 || delay > maxDelayStageDelay {
		renderError(ErrMissingOrInvalidDelay, c)
		return
	}

	request := &model.AddDelayStageRequest{GraphId: graphID, DelayMs: delay,
		CodeLocation: c.GetHeader(protocol.HeaderCodeLocation),
		CallerId:     c.GetHeader(protocol.HeaderCallerRef),
	}
	response, err := s.addStage(request)

	if err != nil {
		renderError(err, c)
		return
	}
	c.Header(protocol.HeaderStageRef, response.StageId)
	c.Status(http.StatusOK)
}

func (s *Server) handleInvokeFunction(c *gin.Context) {
	graphID := c.Param(paramGraphID)
	if !validGraphID(graphID) {
		renderError(ErrInvalidGraphID, c)
		return
	}
	functionID := c.Query("functionId")

	if !validFunctionID(functionID, true) {
		renderError(ErrInvalidFunctionID, c)
		return
	}

	if c.GetHeader(protocol.HeaderDatumType) != protocol.DatumTypeHTTPReq {
		renderError(protocol.ErrInvalidDatumType, c)
		return
	}

	datum, err := protocol.DatumFromRequest(c.Request)

	if err != nil {
		renderError(err, c)
		return
	}
	request := &model.AddInvokeFunctionStageRequest{
		GraphId:      graphID,
		FunctionId:   functionID,
		Arg:          datum.GetHttpReq(),
		CodeLocation: c.GetHeader(protocol.HeaderCodeLocation),
		CallerId:     c.GetHeader(protocol.HeaderCallerRef),
	}

	response, err := s.addStage(request)
	if err != nil {
		renderError(err, c)
		return
	}
	c.Header(protocol.HeaderStageRef, response.StageId)
	c.Status(http.StatusOK)
}

func (s *Server) handleAddTerminationHook(c *gin.Context) {
	graphID := c.Param(paramGraphID)
	if !validGraphID(graphID) {
		renderError(ErrInvalidGraphID, c)
		return
	}

	blob, err := protocol.BlobFromRequest(c.Request)
	if err != nil {
		renderError(err, c)
		return
	}

	request := &model.AddChainedStageRequest{
		GraphId:      graphID,
		Closure:      blob,
		Operation:    model.CompletionOperation_terminationHook,
		Deps:         []string{},
		CodeLocation: c.GetHeader(protocol.HeaderCodeLocation),
		CallerId:     c.GetHeader(protocol.HeaderCallerRef),
	}

	_, err = s.GraphManager.AddStage(request, s.requestTimeout)
	if err != nil {
		renderError(err, c)
		return
	}
	// API does not currently return stage IDs for termination hooks
	c.Status(http.StatusOK)
}

func (s *Server) handlePrometheusMetrics(c *gin.Context) {
	s.promHandler.ServeHTTP(c.Writer, c.Request)
}

func setTracer(ownURL string, zipkinURL string) {
	var (
		debugMode          = false
		serviceName        = "flow-service"
		serviceHostPort    = ownURL
		zipkinHTTPEndpoint = zipkinURL
		// ex: "http://zipkin:9411/api/v1/spans"
	)

	var collector zipkintracer.Collector

	// custom Zipkin collector to send tracing spans to Prometheus
	promCollector, promErr := NewPrometheusCollector()
	if promErr != nil {
		logrus.WithError(promErr).Fatalln("couldn't start Prometheus trace collector")
	}

	logger := zipkintracer.LoggerFunc(func(i ...interface{}) error { logrus.Error(i...); return nil })

	if zipkinHTTPEndpoint != "" {
		// Custom PrometheusCollector and Zipkin HTTPCollector
		httpCollector, zipErr := zipkintracer.NewHTTPCollector(zipkinHTTPEndpoint, zipkintracer.HTTPLogger(logger))
		if zipErr != nil {
			logrus.WithError(zipErr).Fatalln("couldn't start Zipkin trace collector")
		}
		collector = zipkintracer.MultiCollector{httpCollector, promCollector}
	} else {
		// Custom PrometheusCollector only
		collector = promCollector
	}

	ziptracer, err := zipkintracer.NewTracer(zipkintracer.NewRecorder(collector, debugMode, serviceHostPort, serviceName),
		zipkintracer.ClientServerSameSpan(true),
		zipkintracer.TraceID128Bit(true),
	)
	if err != nil {
		logrus.WithError(err).Fatalln("couldn't start tracer")
	}

	// wrap the Zipkin tracer in a FnTracer which will also send spans to Prometheus
	fntracer := NewFnTracer(ziptracer)

	opentracing.SetGlobalTracer(fntracer)
	logrus.WithFields(logrus.Fields{"url": zipkinHTTPEndpoint}).Info("started tracer")
}


// Server is the flow service root
type Server struct {
	Engine         *gin.Engine
	GraphManager   actor.GraphManager
	apiURL         *url.URL
	listen         string
	requestTimeout time.Duration
	promHandler    http.Handler
}

func newEngine(clusterManager *cluster.Manager) *gin.Engine {
	engine := gin.New()
	engine.Use(gin.Logger(), gin.Recovery(), graphCreateInterceptor, clusterManager.ProxyHandler())
	return engine
}

// New creates a new server - params are injected dependencies
func New(clusterManager *cluster.Manager, manager actor.GraphManager,  listenAddress string, maxRequestTimeout time.Duration, zipkinURL string) (*Server, error) {

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

	createGraphApi(s, manager)

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
	if c.Request.URL.Path == "/graph" && len(c.Query(queryParamGraphID)) == 0 {
		UUID, err := uuid.NewRandom()
		if err != nil {
			c.AbortWithError(500, errors.New("Failed to generate UUID for new graph"))
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

func createGraphApi(s *Server, manager actor.GraphManager) {
	s.Engine.GET("/wss", func(c *gin.Context) {
		query.WSSHandler(manager, c.Writer, c.Request)
	})
	graph := s.Engine.Group("/graph")
	{
		graph.POST("", s.handleCreateGraph)
		graph.GET("/:graphId", s.handleGraphState)
		graph.POST("/:graphId/supply", s.handleSupply)
		graph.POST("/:graphId/invokeFunction", s.handleInvokeFunction)
		graph.POST("/:graphId/completedValue", s.handleCompletedValue)
		graph.POST("/:graphId/delay", s.handleDelay)
		graph.POST("/:graphId/allOf", s.handleAllOf)
		graph.POST("/:graphId/anyOf", s.handleAnyOf)
		graph.POST("/:graphId/externalCompletion", s.handleExternalCompletion)
		graph.POST("/:graphId/commit", s.handleCommit)
		graph.POST("/:graphId/terminationHook", s.handleAddTerminationHook)

		stage := graph.Group("/:graphId/stage")
		{
			stage.GET("/:stageId", s.handleGetGraphStage)
			stage.POST("/:stageId/:operation", s.handleStageOperation)
		}
	}
}