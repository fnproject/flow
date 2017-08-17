package main

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/fnproject/completer/actor"
	"github.com/fnproject/completer/model"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

const (
	headerDatumType    string = "FnProject-DatumType"
	headerResultStatus string = "FnProject-ResultStatus"
	headerErrorType    string = "FnProject-ErrorType"
	headerStageID      string = "FnProject-StageID"
	headerHeaderPrefix string = "FnProject-Header"
	headerMethod       string = "FnProject-Method"
	headerResultCode   string = "FnProject-ResultCode"
)

var log = logrus.WithField("logger", "api")

var graphManager actor.GraphManager

func noOpHandler(c *gin.Context) {
	c.Status(http.StatusNotFound)
}

func stageHandler(c *gin.Context) {
	graphID := c.Param("graphId")
	stageID := c.Param("stageId")
	operation := c.Param("operation")
	switch operation {
	case "complete":
		log.Info("Completing stage " + stageID)
		noOpHandler(c)
	case "fail":
		log.Info("Failing stage " + stageID)
		noOpHandler(c)
	default:
		other := c.Query("other")
		cids := []string{stageID}
		if other != "" {
			cids = append(cids, other)
		}

		completionOperation, found := model.CompletionOperation_value[operation]
		if !found {
			c.Status(http.StatusBadRequest)
			return
		}

		body, err := c.GetRawData()
		if err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}

		request := withClosure(graphID, cids, model.CompletionOperation(completionOperation), body)
		response, err := addStage(&request)
		if err != nil {
			c.Status(500)
			return
		}

		c.JSON(http.StatusCreated, response)
	}
}

func createGraphHandler(c *gin.Context) {
	log.Info("Creating graph")
	functionID := c.Query("functionId")

	if functionID == "" {
		c.Status(http.StatusBadRequest)
		return
	}

	graphID, err := uuid.NewRandom()
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}

	req := &model.CreateGraphRequest{FunctionId: functionID, GraphId: graphID.String()}

	f := graphManager.CreateGraph(req, 5*time.Second)

	result, err := f.Result()
	if err != nil {
		c.Status(500)
		return
	}
	resp := result.(*model.CreateGraphResponse)
	c.Header("FnProject-threadid", resp.GraphId)
	c.Status(http.StatusCreated)

}

func getFakeGraphStateResponse(req model.GetGraphStateRequest) model.GetGraphStateResponse {
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

func getGraphState(c *gin.Context) {

	graphID := c.Param("graphId")
	log.Info("Requested graph with Id " + graphID)

	request := model.GetGraphStateRequest{GraphId: graphID}

	// TODO: send to the GraphManager
	c.JSON(http.StatusOK, getFakeGraphStateResponse(request))
}

func getFakeStageResultResponse(request model.GetStageResultRequest) model.GetStageResultResponse {
	return model.GetStageResultResponse{
		GraphId: request.GraphId,
		StageId: request.StageId,
		Result: &model.CompletionResult{
			Successful: true,
			Datum:      model.NewEmptyDatum(),
		},
	}
}

func resultStatus(result *model.CompletionResult) string {
	if result.GetSuccessful() {
		return "success"
	}
	return "failure"
}

func getGraphStage(c *gin.Context) {
	graphID := c.Param("graphId")
	stageID := c.Param("stageId")

	request := model.GetStageResultRequest{
		GraphId: graphID,
		StageId: stageID,
	}

	f := graphManager.GetStageResult(&request, 5*time.Second)

	res, err := f.Result()
	if err != nil {
		c.Status(500)
		return
	}
	response := res.(*model.GetStageResultResponse)

	c.Header("FnProject-threadid", response.GraphId)

	// TODO: send to the GraphManager

	//	response := getFakeStageResultResponse(request)

	result := response.GetResult()
	if result == nil {
		c.Status(http.StatusInternalServerError)
		return
	}

	datum := result.GetDatum()
	if datum == nil {
		c.Status(http.StatusInternalServerError)
		return
	}

	val := datum.GetVal()
	if val == nil {
		c.Status(http.StatusInternalServerError)
		return
	}

	switch v := val.(type) {

	case *model.Datum_Error:
		c.Header(headerDatumType, "error")
		c.Header(headerResultStatus, resultStatus(result))
		error := v.Error
		c.Header(headerErrorType, model.ErrorDatumType_name[int32(error.GetType())])
		c.String(http.StatusOK, error.GetMessage())
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
		c.Status(http.StatusInternalServerError)
		return
	}
}

func acceptExternalCompletion(c *gin.Context) {
	graphID := c.Param("graphId")

	_ = model.AddExternalCompletionStageRequest{GraphId: graphID}
	// TODO: send to the GraphManager
	response := model.AddStageResponse{GraphId: graphID, StageId: "5000"}

	c.JSON(http.StatusCreated, response)
}

func allOrAnyOf(c *gin.Context, op model.CompletionOperation) {
	cidList := c.Query("cids")
	graphID := c.Param("graphId")

	if cidList == "" {
		c.Status(http.StatusBadRequest)
		return
	}

	cids := strings.Split(cidList, ",")

	log.Infof("Adding chained stage type %s, cids %s", op, cids)

	_ = model.AddChainedStageRequest{
		GraphId:   graphID,
		Operation: op,
		Closure:   nil,
		Deps:      cids,
	}

	// TODO: send to the GraphManager
	response := model.AddStageResponse{GraphId: graphID, StageId: "5000"}

	c.JSON(http.StatusCreated, response)
}

func acceptAllOf(c *gin.Context) {
	allOrAnyOf(c, model.CompletionOperation_allOf)
}

func acceptAnyOf(c *gin.Context) {
	allOrAnyOf(c, model.CompletionOperation_anyOf)
}

func supply(c *gin.Context) {
	graphID := c.Param("graphId")

	body, err := c.GetRawData()
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}

	var cids []string

	_ = withClosure(graphID, cids, model.CompletionOperation_supply, body)

	// TODO: send to the GraphManager
	response := model.AddStageResponse{GraphId: graphID, StageId: "5000"}

	c.JSON(http.StatusCreated, response)
}

func completedValue(c *gin.Context) {
	graphID := c.Param("graphId")

	body, err := c.GetRawData()
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}

	result := model.CompletionResult{
		Successful: true,
		Datum:      model.NewBlobDatum(model.NewBlob("application/java-serialized-object", body)),
	}

	request := model.AddCompletedValueStageRequest{
		GraphId: graphID,
		Result:  &result,
	}

	response, err := addStage(&request)
	if err != nil {
		c.Status(500)
		return
	}
	c.JSON(http.StatusCreated, response)
}

func addStage(request interface{}) (*model.AddStageResponse, error) {
	f := graphManager.AddStage(request, 5*time.Second)

	res, err := f.Result()

	return res.(*model.AddStageResponse), err
}

func commitGraph(c *gin.Context) {
	graphID := c.Param("graphId")
	_ = model.CommitGraphRequest{GraphId: graphID}

	// TODO: Send to GraphManager

	response := model.CommitGraphProcessed{GraphId: graphID}
	c.JSON(http.StatusOK, response)
}

func delay(c *gin.Context) {
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

	_ = model.AddDelayStageRequest{GraphId: graphID, DelayMs: delay}

	// TODO: Send to GraphManager

	response := model.AddStageResponse{GraphId: graphID, StageId: "5000"}
	c.JSON(http.StatusOK, response)
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

func invokeFunction(c *gin.Context) {
	graphID := c.Param("graphId")

	functionID := c.Query("functionId")
	if functionID == "" {
		log.Info("Empty or missing functionId supplied to add invokeFunction stage")
		c.Status(http.StatusBadRequest)
		return
	}

	body, err := c.GetRawData()
	if err != nil {
		log.Info("Invalid request body supplied to add invokeFunction stage")
		c.Status(http.StatusBadRequest)
		return
	}

	var method model.HttpMethod
	if m, found := model.HttpMethod_value[c.Request.Method]; found {
		method = model.HttpMethod(m)
	} else {
		method = model.HttpMethod_unknown_method
	}

	_ = model.InvokeFunctionRequest{
		GraphId:    graphID,
		StageId:    "",
		FunctionId: functionID,
		Arg: &model.HttpReqDatum{
			Body:    model.NewBlob(c.ContentType(), body),
			Headers: unwrapPrefixedHeaders(c.Request.Header),
			Method:  method,
		},
	}

	// TODO: Send to GraphManager

	response := model.AddStageResponse{GraphId: graphID, StageId: "5000"}

	c.JSON(http.StatusCreated, response)
}

func withClosure(graphID string, cids []string, op model.CompletionOperation, body []byte) model.AddChainedStageRequest {
	log.Info(fmt.Sprintf("Adding chained stage type %s, cids %s", op, cids))

	return model.AddChainedStageRequest{
		GraphId:   graphID,
		Operation: op,
		Closure:   model.NewBlob("application/java-serialized-object", body),
		Deps:      cids,
	}
}

func main() {

	graphManager = actor.NewGraphManager()
	engine := gin.Default()

	engine.GET("/ping", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	graph := engine.Group("/graph")
	{
		graph.POST("", createGraphHandler)
		graph.GET("/:graphId", getGraphState)

		graph.POST("/:graphId/supply", supply)
		graph.POST("/:graphId/invokeFunction", invokeFunction)
		graph.POST("/:graphId/completedValue", completedValue)
		graph.POST("/:graphId/delay", delay)
		graph.POST("/:graphId/allOf", acceptAllOf)
		graph.POST("/:graphId/anyOf", acceptAnyOf)
		graph.POST("/:graphId/externalCompletion", acceptExternalCompletion)
		graph.POST("/:graphId/commit", commitGraph)

		stage := graph.Group("/:graphId/stage")
		{
			stage.GET("/:stageId", getGraphStage)
			stage.POST("/:stageId/:operation", stageHandler)
		}
	}

	log.Info("Starting")

	listenHost := os.Getenv("COMPLETER_HOST")
	var listenPort = os.Getenv("COMPLETER_PORT")
	if listenPort == "" {
		listenPort = "8081"
	}
	engine.Run(listenHost + ":" + listenPort)
}
