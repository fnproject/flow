package server


import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/fnproject/completer/actor"
	"github.com/fnproject/completer/model"
	"github.com/fnproject/completer/query"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

const (
	headerDatumType    string = "FnProject-DatumType"
	headerResultStatus string = "FnProject-ResultStatus"
	headerErrorType    string = "FnProject-ErrorType"
	headerThreadID     string = "FnProject-ThreadID"
	headerStageID      string = "FnProject-StageID"
	headerHeaderPrefix string = "FnProject-Header"
	headerMethod       string = "FnProject-Method"
	headerResultCode   string = "FnProject-ResultCode"
)

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
	if methodValue, found := model.HttpMethod_value[method]; found {
		m = model.HttpMethod(methodValue)
	} else {
		m = model.HttpMethod_unknown_method
	}

	httpReqDatum := model.HttpReqDatum{
		Body:    model.NewBlob(contentType, body),
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

	f := s.graphManager.CompleteStageExternally(&request, 5*time.Second)

	res, err := f.Result()

	response := res.(*model.CompleteStageExternallyResponse)

	return response, err
}

func (s *Server) stageHandler(c *gin.Context) {
	graphID := c.Param("graphId")
	stageID := c.Param("stageId")
	operation := c.Param("operation")
	body, err := c.GetRawData()
	if err != nil {
		message := "stageHandler can't get raw data from the request"
		log.WithError(err).Error(message)
		c.Status(http.StatusInternalServerError)
		return
	}

	switch operation {
	case "complete":
		response, err := s.completeExternally(graphID, stageID, body, c.Request.Header, c.Request.Method, c.ContentType(), true)
		if err != nil {
			message := "completeExternally returned an error trying to complete with success"
			log.WithError(err).Error(message)
			c.Status(http.StatusInternalServerError)
			return
		}
		c.Header(headerStageID, response.StageId)
		c.Status(http.StatusOK)
	case "fail":
		response, err := s.completeExternally(graphID, stageID, body, c.Request.Header, c.Request.Method, c.ContentType(), false)
		if err != nil {
			message := "completeExternally returned an error trying to complete with failure"
			log.WithError(err).Error(message)
			c.Status(http.StatusInternalServerError)
			return
		}
		c.Header(headerStageID, response.StageId)
		c.Status(http.StatusOK)
	default:
		other := c.Query("other")
		cids := []string{stageID}
		if other != "" {
			cids = append(cids, other)
		}

		completionOperation, found := model.CompletionOperation_value[operation]
		if !found {
			message := "invalid completion operation '" + operation + "'"
			c.Data(http.StatusBadRequest, "text/plain", []byte(message))
			return
		}

		request := withClosure(graphID, cids, model.CompletionOperation(completionOperation), body, c.ContentType())
		response, err := s.addStage(&request)
		if err != nil {
			message := "stageHandler got an error from addStage"
			log.WithError(err).Error(message)
			c.Status(http.StatusInternalServerError)
			return
		}
		c.Header(headerStageID, response.StageId)
		c.Status(http.StatusOK)
	}
}

func (s *Server) createGraphHandler(c *gin.Context) {
	log.Info("Creating graph")
	functionID := c.Query("functionId")

	if functionID == "" {
		c.Status(http.StatusBadRequest)
		return
	}

	graphID, err := uuid.NewRandom()
	if err != nil {
		message := "can't get a UUID for the new graph"
		log.WithError(err).Error(message)
		c.Status(http.StatusInternalServerError)
		return
	}

	req := &model.CreateGraphRequest{FunctionId: functionID, GraphId: graphID.String()}

	f := s.graphManager.CreateGraph(req, 5*time.Second)

	result, err := f.Result()
	if err != nil {
		message := "failed to create new graph"
		log.WithError(err).Error(message)
		c.Status(http.StatusInternalServerError)
		return
	}
	resp := result.(*model.CreateGraphResponse)
	c.Header(headerThreadID, resp.GraphId)
	c.Status(http.StatusOK)
}

func (s *Server) getFakeGraphStateResponse(req model.GetGraphStateRequest) model.GetGraphStateResponse {
	// TODO: delete this, obviously

	stage0 := model.GetGraphStateResponse_StageRepresentation{
		Type:         model.CompletionOperation_name[int32(model.CompletionOperation_delay)],
		Status:       "success",
		Dependencies: []string{},
	}

	stage1 := model.GetGraphStateResponse_StageRepresentation{
		Type:         model.CompletionOperation_name[int32(model.CompletionOperation_delay)],
		Status:       "failure",
		Dependencies: []string{"0"},
	}

	stage2 := model.GetGraphStateResponse_StageRepresentation{
		Type:         model.CompletionOperation_name[int32(model.CompletionOperation_allOf)],
		Status:       "pending",
		Dependencies: []string{"0", "1"},
	}

	response := model.GetGraphStateResponse{
		FunctionId: "theFunctionId",
		GraphId:    req.GraphId,
		Stages: map[string]*model.GetGraphStateResponse_StageRepresentation{
			"0": &stage0,
			"1": &stage1,
			"2": &stage2,
		},
	}

	return response
}

func (s *Server) getGraphState(c *gin.Context) {

	graphID := c.Param("graphId")
	log.Info("Requested graph with Id " + graphID)

	request := model.GetGraphStateRequest{GraphId: graphID}

	// TODO: send to the GraphManager
	c.JSON(http.StatusOK, s.getFakeGraphStateResponse(request))
}

func resultStatus(result *model.CompletionResult) string {
	if result.GetSuccessful() {
		return "success"
	}
	return "failure"
}

func (s *Server) getGraphStage(c *gin.Context) {
	graphID := c.Param("graphId")
	stageID := c.Param("stageId")

	request := model.GetStageResultRequest{
		GraphId: graphID,
		StageId: stageID,
	}

	f := s.graphManager.GetStageResult(&request, 5*time.Second)

	res, err := f.Result()
	if err != nil {
		message := "GetStageResult future returned an error"
		log.WithError(err).Error(message)
		c.Data(http.StatusInternalServerError, "text/plain", []byte(message + "\n" + err.Error()))
		return
	}
	response := res.(*model.GetStageResultResponse)

	result := response.GetResult()
	if result == nil {
		log.Info("CompletionResult in response is nil. Perhaps the stage hasn't completed yet.")
		c.Status(http.StatusPartialContent)
		return
	}

	datum := result.GetDatum()
	if datum == nil {
		message := "GetStageResult produced a result but the datum is null"
		log.Error(message)
		c.Data(http.StatusInternalServerError, "text/plain", []byte(message))
		return
	}

	val := datum.GetVal()
	if val == nil {
		message := "GetStageResult produced a result but the datum value is null"
		log.Error(message)
		c.Data(http.StatusInternalServerError, "text/plain", []byte(message))
		return
	}

	switch v := val.(type) {

	case *model.Datum_Error:
		c.Header(headerDatumType, "error")
		c.Header(headerResultStatus, resultStatus(result))
		err := v.Error
		c.Header(headerErrorType, model.ErrorDatumType_name[int32(err.GetType())])
		c.String(http.StatusOK, err.GetMessage())
		return
	case *model.Datum_Empty:
		c.Header(headerDatumType, "empty")
		c.Header(headerResultStatus, resultStatus(result))
		c.Status(http.StatusOK)
		return
	case *model.Datum_Blob:
		c.Header(headerDatumType, "blob")
		c.Header(headerResultStatus, resultStatus(result))
		blob := v.Blob
		c.Data(http.StatusOK, blob.GetContentType(), blob.GetDataString())
		return
	case *model.Datum_StageRef:
		c.Header(headerDatumType, "stageref")
		c.Header(headerResultStatus, resultStatus(result))
		stageRef := v.StageRef
		c.Header(headerStageID, stageRef.StageRef)
		c.Status(http.StatusOK)
		return
	case *model.Datum_HttpReq:
		c.Header(headerDatumType, "httpreq")
		c.Header(headerResultStatus, resultStatus(result))
		httpReq := v.HttpReq
		for _, header := range httpReq.Headers {
			c.Header(headerHeaderPrefix+"-"+header.GetKey(), header.GetValue())
		}
		httpMethod := model.HttpMethod_name[int32(httpReq.GetMethod())]
		c.Header(headerMethod, httpMethod)
		c.Data(http.StatusOK, httpReq.Body.GetContentType(), httpReq.Body.GetDataString())
		return
	case *model.Datum_HttpResp:
		c.Header(headerDatumType, "httpresp")
		c.Header(headerResultStatus, resultStatus(result))
		httpResp := v.HttpResp
		for _, header := range httpResp.Headers {
			c.Header(headerHeaderPrefix+"-"+header.GetKey(), header.GetValue())
		}
		statusCode := strconv.FormatUint(uint64(httpResp.GetStatusCode()), 32)
		c.Header(headerResultCode, statusCode)
		c.Data(http.StatusOK, httpResp.Body.GetContentType(), httpResp.Body.GetDataString())
		return
	default:
		log.Error("unrecognized datum type when getting graph stage")
		c.Status(http.StatusInternalServerError)
		return
	}
}

func (s *Server) acceptExternalCompletion(c *gin.Context) {
	graphID := c.Param("graphId")

	request := model.AddExternalCompletionStageRequest{GraphId: graphID}

	response, err := s.addStage(&request)

	if err != nil {
		message := "acceptExternalCompletion failed to add stage"
		log.WithError(err).Error(message)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.Header(headerStageID, response.StageId)
	c.Status(http.StatusOK)
}

func (s *Server) allOrAnyOf(c *gin.Context, op model.CompletionOperation) {
	cidList := c.Query("cids")
	graphID := c.Param("graphId")

	if cidList == "" {
		c.Status(http.StatusBadRequest)
		return
	}

	cids := strings.Split(cidList, ",")

	request := model.AddChainedStageRequest{
		GraphId:   graphID,
		Operation: op,
		Closure:   nil,
		Deps:      cids,
	}

	response, err := s.addStage(&request)

	if err != nil {
		message := "allOrAnyOf failed to add stage"
		log.WithError(err).Error(message)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.Header(headerStageID, response.StageId)
	c.Status(http.StatusOK)
}

func (s *Server) acceptAllOf(c *gin.Context) {
	s.allOrAnyOf(c, model.CompletionOperation_allOf)
}

func (s *Server) acceptAnyOf(c *gin.Context) {
	s.allOrAnyOf(c, model.CompletionOperation_anyOf)
}

func (s *Server) supply(c *gin.Context) {
	graphID := c.Param("graphId")

	body, err := c.GetRawData()
	if err != nil {
		message := "supply cannot get raw request data"
		log.WithError(err).Error(message)
		c.Status(http.StatusInternalServerError)
		return
	}

	var cids []string

	request := withClosure(graphID, cids, model.CompletionOperation_supply, body, c.ContentType())

	response, err := s.addStage(&request)
	if err != nil {
		message := "supply failed to add stage"
		log.WithError(err).Error(message)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.Header(headerStageID, response.StageId)
	c.Status(http.StatusOK)
}

func (s *Server) completedValue(c *gin.Context) {
	graphID := c.Param("graphId")

	body, err := c.GetRawData()
	if err != nil {
		message := "completedValue cannot get raw request data"
		log.WithError(err).Error(message)
		c.Status(http.StatusInternalServerError)
		return
	}

	result := model.CompletionResult{
		Successful: true,
		Datum:      model.NewBlobDatum(model.NewBlob(c.ContentType(), body)),
	}

	request := model.AddCompletedValueStageRequest{
		GraphId: graphID,
		Result:  &result,
	}

	response, err := s.addStage(&request)
	if err != nil {
		message := "completedValue failed to add stage"
		log.WithError(err).Error(message)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.Header(headerStageID, response.StageId)
	c.Status(http.StatusOK)
}

func (s *Server) addStage(request interface{}) (*model.AddStageResponse, error) {
	f := s.graphManager.AddStage(request, 5*time.Second)

	res, err := f.Result()

	return res.(*model.AddStageResponse), err
}

func (s *Server) commitGraph(c *gin.Context) {
	graphID := c.Param("graphId")
	request := model.CommitGraphRequest{GraphId: graphID}

	f := s.graphManager.Commit(&request, 5*time.Second)

	result, err := f.Result()
	if err != nil {
		message := "commitGraph failed to commit"
		log.WithError(err).Error(message)
		c.Status(http.StatusInternalServerError)
		return
	}

	response := result.(*model.CommitGraphProcessed)
	c.Header(headerThreadID, response.GraphId)
	c.Status(http.StatusOK)
}

func (s *Server) delay(c *gin.Context) {
	graphID := c.Param("graphId")
	delayMs := c.Query("delayMs")
	if delayMs == "" {
		log.Info("Empty or missing delay value supplied to add delay stage")
		c.Status(http.StatusBadRequest)
		return
	}

	delay, err := strconv.ParseInt(delayMs, 10, 64)
	if err != nil {
		log.Info("Invalid delay value supplied to add delay stage")
		c.Status(http.StatusBadRequest)
		return
	}

	request := model.AddDelayStageRequest{GraphId: graphID, DelayMs: delay}

	response, err := s.addStage(&request)

	if err != nil {
		message := "delay failed to add stage"
		log.WithError(err).Error(message)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.Header(headerStageID, response.StageId)
	c.Status(http.StatusOK)
}

func unwrapPrefixedHeaders(hs http.Header) []*model.HttpHeader {
	var headers []*model.HttpHeader
	for k, vs := range hs {
		canonicalKey := http.CanonicalHeaderKey(k)
		canonicalPrefix := http.CanonicalHeaderKey(headerHeaderPrefix + "-")
		if strings.HasPrefix(canonicalKey, canonicalPrefix) {
			trimmedHeader := strings.TrimPrefix(canonicalKey, canonicalPrefix)
			for _, v := range vs {
				headers = append(headers, &model.HttpHeader{
					Key:   trimmedHeader,
					Value: v,
				})
			}
		}
	}
	return headers
}

func (s *Server) invokeFunction(c *gin.Context) {
	graphID := c.Param("graphId")

	functionID := c.Query("functionId")
	if functionID == "" {
		message := "Empty or missing functionId supplied to add invokeFunction stage"
		log.Info(message)
		c.Data(http.StatusBadRequest, "text/plain", []byte(message))
		return
	}

	body, err := c.GetRawData()
	if err != nil {
		message := "Invalid request body supplied to add invokeFunction stage"
		log.Info(message)
		c.Data(http.StatusBadRequest, "text/plain", []byte(message))
		return
	}

	requestMethod := strings.ToLower(c.GetHeader(http.CanonicalHeaderKey(headerMethod)))
	var method model.HttpMethod
	if m, found := model.HttpMethod_value[requestMethod]; found {
		method = model.HttpMethod(m)
	} else {
		message := "Empty, missing, or invalid HTTP method supplied to add invokeFunction stage"
		log.Info(message)
		c.Data(http.StatusBadRequest, "text/plain", []byte(message))
		return
	}

	request := model.AddInvokeFunctionStageRequest{
		GraphId:    graphID,
		FunctionId: functionID,
		Arg: &model.HttpReqDatum{
			Body:    model.NewBlob(c.ContentType(), body),
			Headers: unwrapPrefixedHeaders(c.Request.Header),
			Method:  method,
		},
	}

	response, err := s.addStage(&request)
	if err != nil {
		log.WithError(err).Error("invokeFunction failed to add stage")
		c.Status(http.StatusInternalServerError)
		return
	}
	c.Header(headerStageID, response.StageId)
	c.Status(http.StatusOK)
}

func withClosure(graphID string, cids []string, op model.CompletionOperation, body []byte, contentType string) model.AddChainedStageRequest {
	log.Info(fmt.Sprintf("Adding chained stage type %s, cids %s", op, cids))

	return model.AddChainedStageRequest{
		GraphId:   graphID,
		Operation: op,
		Closure:   model.NewBlob(contentType, body),
		Deps:      cids,
	}
}

type Server struct {
	engine *gin.Engine
	graphManager actor.GraphManager
	listenHost string
	listenPort string
}

func NewServer(listenHost string, listenPort string, manager actor.GraphManager) (*Server, error) {
	s := &Server{
		graphManager: manager,
		engine: gin.Default(),
		listenHost: listenHost,
		listenPort: listenPort,
	}

	s.engine.GET("/ping", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	s.engine.GET("/wss", func(c *gin.Context) {
		query.WSSHandler(c.Writer, c.Request)
	})

	graph := s.engine.Group("/graph")
	{
		graph.POST("", s.createGraphHandler)
		graph.GET("/:graphId", s.getGraphState)

		graph.POST("/:graphId/supply", s.supply)
		graph.POST("/:graphId/invokeFunction", s.invokeFunction)
		graph.POST("/:graphId/completedValue", s.completedValue)
		graph.POST("/:graphId/delay", s.delay)
		graph.POST("/:graphId/allOf", s.acceptAllOf)
		graph.POST("/:graphId/anyOf", s.acceptAnyOf)
		graph.POST("/:graphId/externalCompletion", s.acceptExternalCompletion)
		graph.POST("/:graphId/commit", s.commitGraph)

		stage := graph.Group("/:graphId/stage")
		{
			stage.GET("/:stageId", s.getGraphStage)
			stage.POST("/:stageId/:operation", s.stageHandler)
		}
	}

	log.Info("Starting")

	if listenPort == "" {
		listenPort = "8081"
	}


	return s, nil
}

func (s *Server) Run() {
	s.engine.Run(s.listenHost + ":" + s.listenPort)
}