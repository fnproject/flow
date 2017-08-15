package main

import (
	"github.com/fnproject/completer/model"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"net/http"
	"strconv"
	"strings"
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

func acceptExternalCompletion(c *gin.Context) {

	graphID := c.Param("graphId")

	_ = model.AddExternalCompletionStageRequest{GraphId: graphID}
	// TODO: send to the GraphManager
	response := model.AddStageResponse{GraphId: graphID, StageId: 5000}

	c.JSON(http.StatusCreated, response)
}

func allOrAnyOf(c *gin.Context, op model.CompletionOperation) {
	cidList := c.Query("cids")
	graphID := c.Param("graphId")

	if cidList == "" {
		c.Status(http.StatusBadRequest)
		return
	}

	var cids []uint32
	for _, c := range strings.Split(cidList, ",") {
		cid, _ := strconv.Atoi(c) // TODO: no error handling
		cids = append(cids, uint32(cid))
	}

	log.Infof("Adding chained stage type %s, cids %s", op, cids)

	_ = model.AddChainedStageRequest{
		GraphId:   graphID,
		Operation: op,
		Closure:   nil,
		Deps:      cids,
	}

	// TODO: send to the GraphManager
	response := model.AddStageResponse{GraphId: graphID, StageId: 5000}

	c.JSON(http.StatusCreated, response)
}

func acceptAllOf(c *gin.Context) {
	allOrAnyOf(c, model.CompletionOperation_allOf)
}

func acceptAnyOf(c *gin.Context) {
	allOrAnyOf(c, model.CompletionOperation_anyOf)
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
		graph.POST("/:graphId/allOf", acceptAllOf)
		graph.POST("/:graphId/anyOf", acceptAnyOf)
		graph.POST("/:graphId/externalCompletion", acceptExternalCompletion)
		graph.POST("/:graphId/commit", noOpHandler)

		stage := graph.Group("/:graphId/stage")
		{
			stage.GET("/:stageId", noOpHandler)
			stage.POST("/:stageId/:operation", stageHandler)
		}
	}

	log.Info("Starting")

	engine.Run()
}
