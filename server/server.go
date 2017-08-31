package server

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"fmt"
	"net/url"

	protoactor "github.com/AsynkronIT/protoactor-go/actor"
	"github.com/fnproject/completer/actor"
	"github.com/fnproject/completer/model"
	"github.com/fnproject/completer/persistence"
	"github.com/fnproject/completer/protocol"
	"github.com/fnproject/completer/query"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

const MaxDelay = 3600 * 1000 * 24

var log = logrus.WithField("logger", "server")

func (s *Server) completeExternally(graphID string, stageID string, body []byte, headers http.Header, method string, contentType string, b bool) (*model.CompleteStageExternallyResponse, error) {
	var hs []*model.HttpHeader
	for k, vs := range headers {
		for _, v := range vs {
			hs = append(hs, &model.HttpHeader{
				Key:   k,
				Value: v,
			})
		}
	}

	var m model.HttpMethod
	if methodValue, found := model.HttpMethod_value[strings.ToLower(method)]; found {
		m = model.HttpMethod(methodValue)
	} else {
		return nil, ErrUnsupportedHttpMethod
	}

	if contentType == "" {
		contentType = "application/octet-stream"
	}

	blob, err := s.BlobStore.CreateBlob(contentType, body)
	if err != nil {
		return nil, err
	}
	httpReqDatum := model.HttpReqDatum{
		Body:    blob,
		Headers: hs,
		Method:  m,
	}

	request := model.CompleteStageExternallyRequest{
		GraphId: graphID,
		StageId: stageID,
		Result: &model.CompletionResult{
			Successful: b,
			Datum:      model.NewHttpReqDatum(&httpReqDatum),
		},
	}

	response, err := s.GraphManager.CompleteStageExternally(&request, s.requestTimeout)

	return response, err
}

func (s *Server) handleStageOperation(c *gin.Context) {
	graphID := c.Param("graphId")
	if !validGraphId(graphID) {
		renderError(ErrInvalidGraphId, c)
		return
	}
	stageID := c.Param("stageId")
	if !validStageId(graphID) {
		renderError(ErrInvalidStageId, c)
		return
	}
	operation := c.Param("operation")
	body, err := c.GetRawData()
	if err != nil {
		renderError(ErrReadingInput, c)
		return
	}

	switch operation {
	case "complete":
		response, err := s.completeExternally(graphID, stageID, body, c.Request.Header, c.Request.Method, c.ContentType(), true)
		if err != nil {
			renderError(err, c)
			return
		}
		c.Header(protocol.HeaderStageRef, response.StageId)
		c.Status(http.StatusOK)
	case "fail":
		response, err := s.completeExternally(graphID, stageID, body, c.Request.Header, c.Request.Method, c.ContentType(), false)
		if err != nil {
			renderError(err, c)
			return
		}
		c.Header(protocol.HeaderStageRef, response.StageId)
		c.Status(http.StatusOK)
	default:
		other := c.Query("other")
		cids := []string{stageID}
		if other != "" {
			if !validStageId(other) {
				renderError(ErrInvalidDepStageId, c)
				return
			}
			cids = append(cids, other)
		}

		completionOperation, found := model.CompletionOperation_value[operation]
		if !found {
			renderError(ErrUnrecognisedCompletionOperation, c)
			return

		}

		blob, err := s.BlobStore.CreateBlob(c.ContentType(), body)
		if err != nil {
			renderError(err, c)
			return
		}

		// TODO: enforce valid content type
		// TODO: generic error handling
		request := &model.AddChainedStageRequest{
			GraphId:   graphID,
			Deps:      cids,
			Operation: model.CompletionOperation(completionOperation),
			Closure:   blob,
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
		log.WithError(err).Error("Error occured in request")
	}
	switch e := err.(type) {

	case model.ValidationError, *protocol.BadProtoMessage:
		c.Data(http.StatusBadRequest, "text/plain", []byte(e.Error()))
	case *ServerErr:
		{
			c.Data(e.HttpStatus, "text/plain", []byte(e.Message))
		}
	default:
		log.WithError(err).Error("Internal server error")
		c.Status(http.StatusInternalServerError)
	}
}

func (s *Server) handleCreateGraph(c *gin.Context) {
	log.Info("Creating graph")
	functionID := c.Query("functionId")

	if !validFunctionId(functionID) {
		log.WithField("function_id", functionID).Info("Invalid function iD ")
		renderError(ErrInvalidFunctionId, c)
		return
	}
	// TODO: Validate the format of the functionId

	graphID, err := uuid.NewRandom()
	if err != nil {
		renderError(err, c)
		return
	}

	req := &model.CreateGraphRequest{FunctionId: functionID, GraphId: graphID.String()}

	// TODO: sort out timeouts in a consistent way
	result, err := s.GraphManager.CreateGraph(req, s.requestTimeout)
	if err != nil {
		renderError(err, c)
		return
	}
	c.Header(protocol.HeaderThreadId, result.GraphId)
	c.Status(http.StatusOK)
}

func (s *Server) handleGraphState(c *gin.Context) {

	graphID := c.Param("graphId")
	if !validGraphId(graphID) {
		renderError(ErrInvalidGraphId, c)
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

func resultStatus(result *model.CompletionResult) string {
	if result.GetSuccessful() {
		return "success"
	}
	return "failure"
}

func (s *Server) handleGetGraphStage(c *gin.Context) {
	graphID := c.Param("graphId")
	stageID := c.Param("stageId")

	if !validGraphId(graphID) {
		renderError(ErrInvalidGraphId, c)
		return
	}
	if !validStageId(stageID) {
		renderError(ErrInvalidStageId, c)
		return
	}

	request := model.GetStageResultRequest{
		GraphId: graphID,
		StageId: stageID,
	}

	response, err := s.GraphManager.GetStageResult(&request, s.requestTimeout)

	if err == protoactor.ErrTimeout {
		c.Data(http.StatusRequestTimeout, "text/plain", []byte("stage not completed"))
		return
	}

	if err != nil {
		renderError(err, c)
		return
	}

	result := response.GetResult()
	datum := result.GetDatum()
	val := datum.GetVal()

	switch v := val.(type) {

	// TODO: refactor this by adding a writer to a context in proto/write.go
	case *model.Datum_Error:
		c.Header(protocol.HeaderDatumType, protocol.DatumTypeError)
		c.Header(protocol.HeaderResultStatus, resultStatus(result))
		err := v.Error
		c.Header(protocol.HeaderErrorType, model.ErrorDatumType_name[int32(err.GetType())])
		c.String(http.StatusOK, err.GetMessage())
		return
	case *model.Datum_Empty:
		c.Header(protocol.HeaderDatumType, protocol.DatumTypeEmpty)
		c.Header(protocol.HeaderResultStatus, resultStatus(result))
		c.Status(http.StatusOK)
		return
	case *model.Datum_Blob:
		blob := v.Blob
		blobData, err := s.BlobStore.ReadBlobData(blob)
		if err != nil {
			renderError(err, c)
			return
		}
		c.Header(protocol.HeaderDatumType, protocol.DatumTypeBlob)
		c.Header(protocol.HeaderResultStatus, resultStatus(result))

		c.Data(http.StatusOK, blob.GetContentType(), blobData)
		return
	case *model.Datum_StageRef:
		c.Header(protocol.HeaderDatumType, protocol.DatumTypeStageRef)
		c.Header(protocol.HeaderResultStatus, resultStatus(result))
		stageRef := v.StageRef
		c.Header(protocol.HeaderStageRef, stageRef.StageRef)
		c.Status(http.StatusOK)
		return
	case *model.Datum_HttpReq:
		httpReq := v.HttpReq
		var body []byte
		if httpReq.Body != nil {
			body, err = s.BlobStore.ReadBlobData(httpReq.Body)
			if err != nil {
				renderError(err, c)
				return
			}
		}

		c.Header(protocol.HeaderDatumType, protocol.DatumTypeHttpReq)
		c.Header(protocol.HeaderResultStatus, resultStatus(result))
		for _, header := range httpReq.Headers {
			c.Header(protocol.HeaderHeaderPrefix+header.GetKey(), header.GetValue())
		}
		httpMethod := model.HttpMethod_name[int32(httpReq.GetMethod())]
		c.Header(protocol.HeaderMethod, httpMethod)
		c.Data(http.StatusOK, httpReq.Body.GetContentType(), body)
		return
	case *model.Datum_HttpResp:
		var body []byte
		httpResp := v.HttpResp

		if httpResp.Body != nil {
			body, err = s.BlobStore.ReadBlobData(httpResp.Body)
			if err != nil {
				renderError(err, c)
				return
			}
		}
		c.Header(protocol.HeaderDatumType, protocol.DatumTypeHttpResp)
		c.Header(protocol.HeaderResultStatus, resultStatus(result))
		for _, header := range httpResp.Headers {
			c.Header(protocol.HeaderHeaderPrefix+header.GetKey(), header.GetValue())
		}
		statusCode := fmt.Sprintf("%d", httpResp.GetStatusCode())
		c.Header(protocol.HeaderResultCode, statusCode)
		c.Data(http.StatusOK, httpResp.Body.GetContentType(), body)
		return
	default:
		log.Error("unrecognized datum type when getting graph stage")
		c.Status(http.StatusInternalServerError)
		return
	}
}

func (s *Server) handleExternalCompletion(c *gin.Context) {
	graphID := c.Param("graphId")
	if !validGraphId(graphID) {
		renderError(ErrInvalidGraphId, c)
		return
	}
	request := &model.AddExternalCompletionStageRequest{GraphId: graphID}

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
	graphID := c.Param("graphId")
	if !validGraphId(graphID) {
		renderError(ErrInvalidGraphId, c)
		return
	}
	cids := strings.Split(cidList, ",")

	for _, stageId := range cids {
		if !validStageId(stageId) {
			renderError(ErrInvalidDepStageId, c)
			return
		}
	}
	request := &model.AddChainedStageRequest{
		GraphId:   graphID,
		Operation: op,
		Closure:   nil,
		Deps:      cids,
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
	graphID := c.Param("graphId")
	if !validGraphId(graphID) {
		renderError(ErrInvalidGraphId, c)
		return
	}
	ct := c.ContentType()
	if ct == "" {
		renderError(protocol.ErrMissingContentType, c)
		return
	}

	body, err := c.GetRawData()
	if err != nil {
		renderError(err, c)
		return
	}
	if len(body) == 0 {
		renderError(ErrMissingBody, c)
		return
	}

	blob, err := s.BlobStore.CreateBlob(ct, body)
	if err != nil {
		renderError(err, c)
		return
	}

	request := &model.AddChainedStageRequest{
		GraphId:   graphID,
		Operation: model.CompletionOperation_supply,
		Closure:   blob,
		Deps:      []string{},
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
	graphID := c.Param("graphId")
	if !validGraphId(graphID) {
		renderError(ErrInvalidGraphId, c)
		return
	}

	datum, err := protocol.DatumFromRequest(s.BlobStore, c.Request)
	if err != nil {
		renderError(err, c)
		return
	}

	result := model.CompletionResult{
		Successful: true,
		Datum:      datum,
	}

	request := &model.AddCompletedValueStageRequest{
		GraphId: graphID,
		Result:  &result,
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
	graphID := c.Param("graphId")
	request := model.CommitGraphRequest{GraphId: graphID}

	response, err := s.GraphManager.Commit(&request, s.requestTimeout)
	if err != nil {
		renderError(err, c)
		return
	}

	c.Header(protocol.HeaderThreadId, response.GraphId)
	c.Status(http.StatusOK)
}

func (s *Server) handleDelay(c *gin.Context) {
	graphID := c.Param("graphId")
	if !validGraphId(graphID) {
		renderError(ErrInvalidGraphId, c)
		return
	}
	delayMs := c.Query("delayMs")
	if delayMs == "" {
		renderError(ErrMissingOrInvalidDelay, c)
		return
	}

	delay, err := strconv.ParseInt(delayMs, 10, 64)
	if err != nil || delay < 0 || delay > MaxDelay {
		renderError(ErrMissingOrInvalidDelay, c)
		return
	}

	request := &model.AddDelayStageRequest{GraphId: graphID, DelayMs: delay}

	response, err := s.addStage(request)

	if err != nil {
		renderError(err, c)
		return
	}
	c.Header(protocol.HeaderStageRef, response.StageId)
	c.Status(http.StatusOK)
}

func (s *Server) handleInvokeFunction(c *gin.Context) {
	graphID := c.Param("graphId")
	if !validGraphId(graphID) {
		renderError(ErrInvalidGraphId, c)
		return
	}
	functionID := c.Query("functionId")

	if !validFunctionId(functionID) {
		renderError(ErrInvalidFunctionId, c)
		return
	}

	if c.GetHeader(protocol.HeaderDatumType) != protocol.DatumTypeHttpReq {
		renderError(protocol.ErrInvalidDatumType, c)
		return
	}

	datum, err := protocol.DatumFromRequest(s.BlobStore, c.Request)

	if err != nil {
		renderError(err, c)
		return
	}
	request := &model.AddInvokeFunctionStageRequest{
		GraphId:    graphID,
		FunctionId: functionID,
		Arg:        datum.GetHttpReq(),
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
	graphID := c.Param("graphId")
	if !validGraphId(graphID) {
		renderError(ErrInvalidGraphId, c)
		return
	}

	body, err := c.GetRawData()
	if err != nil {
		renderError(ErrReadingInput, c)
		return
	}

	blob, err := s.BlobStore.CreateBlob(c.ContentType(), body)
	if err != nil {
		renderError(err, c)
		return
	}

	request := &model.AddTerminationHookRequest{
		GraphId: graphID,
		Closure: blob,
	}

	if _, err := s.GraphManager.AddTerminationHook(request, s.requestTimeout); err != nil {
		renderError(err, c)
		return
	}
	c.Status(http.StatusOK)
}

type Server struct {
	Engine         *gin.Engine
	GraphManager   actor.GraphManager
	apiUrl         *url.URL
	BlobStore      persistence.BlobStore
	listen         string
	requestTimeout time.Duration
}

func New(manager actor.GraphManager, blobStore persistence.BlobStore, listenAddress string) (*Server, error) {

	s := &Server{
		GraphManager:   manager,
		Engine:         gin.Default(),
		listen:         listenAddress,
		BlobStore:      blobStore,
		requestTimeout: 5 * time.Second,
	}

	s.Engine.GET("/ping", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

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

	return s, nil
}

func (s *Server) Run() {
	log.WithField("listen_url", s.listen).Info("Starting Completer server")

	s.Engine.Run(s.listen)
}
