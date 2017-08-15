package main

import (
	"github.com/fnproject/completer/model"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"net/http"
	"strconv"
)

var log = logrus.WithField("logger", "api")

func noOpHandler(c *gin.Context) {
	c.Status(http.StatusNotFound)
}

func stageHandler(c *gin.Context) {
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
		log.Info("Stage operation " + operation)
	}
}

func createGraphHandler(c *gin.Context) {
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

	_ = model.CreateGraphRequest{FunctionId: functionID, GraphId: graphID.String()}
	// TODO: pass this request to the GraphManager
	c.Status(http.StatusCreated)

}

func getFakeGraphStateResponse(req model.GetGraphStateRequest) model.GetGraphStateResponse {
	// TODO: delete this, obviously

	stage0 := model.GetGraphStateResponse_StageRepresentation{
		Type:         model.CompletionOperation_name[int32(model.CompletionOperation_delay)],
		Status:       "success",
		Dependencies: []uint32{},
	}

	stage1 := model.GetGraphStateResponse_StageRepresentation{
		Type:         model.CompletionOperation_name[int32(model.CompletionOperation_delay)],
		Status:       "failure",
		Dependencies: []uint32{0},
	}

	stage2 := model.GetGraphStateResponse_StageRepresentation{
		Type:         model.CompletionOperation_name[int32(model.CompletionOperation_allOf)],
		Status:       "pending",
		Dependencies: []uint32{0, 1},
	}

	response := model.GetGraphStateResponse{
		FunctionId: "theFunctionId",
		GraphId:    req.GraphId,
		Stages: map[uint32]*model.GetGraphStateResponse_StageRepresentation{
			0: &stage0,
			1: &stage1,
			2: &stage2,
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
			Datum: &model.Datum{
				Val: &model.Datum_Empty{
					Empty: &model.EmptyDatum{},
				},
			},
		},
	}
}

func getGraphStage(c *gin.Context) {
	graphID := c.Param("graphId")
	stageID := c.Param("stageId")

	stageNumber, err := strconv.ParseUint(stageID, 10, 32)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}

	request := model.GetStageResultRequest{
		GraphId: graphID,
		StageId: uint32(stageNumber),
	}

	// TODO: send to the GraphManager

	response := getFakeStageResultResponse(request)

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

	myval := datum.GetVal()
	if myval == nil {
		c.Status(http.StatusInternalServerError)
		return
	}

	switch v := myval.(type) {
	case *model.Datum_Error:
		c.Header("FnProject-DatumType", "error")
		resultStatus := "false"
		if result.GetSuccessful() {
			resultStatus = "true"
		}
		c.Header("FnProject-ResultStatus", resultStatus)
		c.Header("FnProject-ErrorType", model.ErrorDatumType_name[int32(v.Error.GetType())])
		c.String(http.StatusOK, v.Error.GetMessage())
		return
	case *model.Datum_Empty:
		c.Header("FnProject-DatumType", "empty")
		resultStatus := "false"
		if result.GetSuccessful() {
			resultStatus = "true"
		}
		c.Header("FnProject-ResultStatus", resultStatus)
		c.Status(http.StatusOK)
		return
	case *model.Datum_Blob:
		c.Header("FnProject-DatumType", "blob")
		resultStatus := "false"
		if result.GetSuccessful() {
			resultStatus = "true"
		}
		c.Header("FnProject-ResultStatus", resultStatus)
		c.Data(http.StatusOK, v.Blob.GetContentType(), v.Blob.GetDataString())
		return
	case *model.Datum_StageRef:
		c.Header("FnProject-DatumType", "stageref")
		resultStatus := "false"
		if result.GetSuccessful() {
			resultStatus = "true"
		}
		c.Header("FnProject-ResultStatus", resultStatus)
		c.Header("FnProject-StageID", strconv.FormatUint(uint64(v.StageRef.StageRef), 32))
		c.Status(http.StatusOK)
		return
	case *model.Datum_HttpReq:
		c.Header("FnProject-DatumType", "httpreq")
		resultStatus := "false"
		if result.GetSuccessful() {
			resultStatus = "true"
		}
		c.Header("FnProject-ResultStatus", resultStatus)

		for _, header := range v.HttpReq.Headers {
			c.Header("FnProject-Header-"+header.GetKey(), header.GetValue())
		}
		c.Header("FnProject-Method", model.HttpMethod_name[int32(v.HttpReq.GetMethod())])
		c.Data(http.StatusOK, v.HttpReq.Body.GetContentType(), v.HttpReq.Body.GetDataString())
		return
	case *model.Datum_HttpResp:
		c.Header("FnProject-DatumType", "httpreq")
		resultStatus := "false"
		if result.GetSuccessful() {
			resultStatus = "true"
		}
		c.Header("FnProject-ResultStatus", resultStatus)
		for _, header := range v.HttpResp.Headers {
			c.Header("FnProject-Header-"+header.GetKey(), header.GetValue())
		}
		c.Header("FnProject-ResultCode", strconv.FormatUint(uint64(v.HttpResp.GetStatusCode()), 32))
		c.Data(http.StatusOK, v.HttpResp.Body.GetContentType(), v.HttpResp.Body.GetDataString())
		return
	default:
		c.Status(http.StatusInternalServerError)
		return
	}
}

func main() {

	engine := gin.Default()

	engine.GET("/ping", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	graph := engine.Group("/graph")
	{
		graph.POST("", createGraphHandler)
		graph.GET("/:graphId", getGraphState)

		graph.POST("/:graphId/supply", noOpHandler)
		graph.POST("/:graphId/invokeFunction", noOpHandler)
		graph.POST("/:graphId/completedValue", noOpHandler)
		graph.POST("/:graphId/delay", noOpHandler)
		graph.POST("/:graphId/allOf", noOpHandler)
		graph.POST("/:graphId/anyOf", noOpHandler)
		graph.POST("/:graphId/externalCompletion", noOpHandler)
		graph.POST("/:graphId/commit", noOpHandler)

		stage := graph.Group("/:graphId/stage")
		{
			stage.GET("/:stageId", getGraphStage)
			stage.POST("/:stageId/:operation", stageHandler)
		}
	}

	log.Info("Starting")

	engine.Run()
}
