package server

import (
	"github.com/gin-gonic/gin"
	"github.com/fnproject/flow/actor"
	"github.com/fnproject/flow/query"
	"github.com/fnproject/flow/protocol"
	protoactor "github.com/AsynkronIT/protoactor-go/actor"
	"strings"
	"fmt"
	"net/http"
	"github.com/fnproject/flow/model"
	"time"
	"strconv"
)


func (s *Server) completeExternally(graphID string, stageID string, body []byte, headers http.Header, method string, contentType string, b bool) (*model.CompleteStageExternallyResponse, error) {
	var hs []*model.HTTPHeader
	for k, vs := range headers {
		for _, v := range vs {
			hs = append(hs, &model.HTTPHeader{
				Key:   k,
				Value: v,
			})
		}
	}

	var m model.HTTPMethod
	if methodValue, found := model.HTTPMethod_value[strings.ToLower(method)]; found {
		m = model.HTTPMethod(methodValue)
	} else {
		return nil, ErrUnsupportedHTTPMethod
	}

	if contentType == "" {
		contentType = "application/octet-stream"
	}

	blob, err := s.BlobStore.CreateBlob(contentType, body)
	if err != nil {
		return nil, err
	}
	httpReqDatum := model.HTTPReqDatum{
		Body:    blob,
		Headers: hs,
		Method:  m,
	}

	request := model.CompleteStageExternallyRequest{
		GraphId: graphID,
		StageId: stageID,
		Result: &model.CompletionResult{
			Successful: b,
			Datum:      model.NewHTTPReqDatum(&httpReqDatum),
		},
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

		blob, err := s.BlobStore.CreateBlob(c.ContentType(), body)
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

func resultStatus(result *model.CompletionResult) string {
	if result.GetSuccessful() {
		return "success"
	}
	return "failure"
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

		c.Header(protocol.HeaderDatumType, protocol.DatumTypeHTTPReq)
		c.Header(protocol.HeaderResultStatus, resultStatus(result))
		for _, header := range httpReq.Headers {
			c.Header(protocol.HeaderHeaderPrefix+header.GetKey(), header.GetValue())
		}
		httpMethod := model.HTTPMethod_name[int32(httpReq.GetMethod())]
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
		c.Header(protocol.HeaderDatumType, protocol.DatumTypeHTTPResp)
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

	result, err := protocol.CompletionResultFromRequest(s.BlobStore, c.Request)
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
		CallerId: c.GetHeader(protocol.HeaderCallerRef),
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

	datum, err := protocol.DatumFromRequest(s.BlobStore, c.Request)

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

	codeLoc := c.GetHeader(protocol.HeaderCodeLocation)

	request := &model.AddChainedStageRequest{
		GraphId:      graphID,
		Closure:      blob,
		Operation:    model.CompletionOperation_terminationHook,
		Deps:         []string{},
		CodeLocation: codeLoc,
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