package main

import (
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"net/http"
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

func main() {

	engine := gin.Default()

	engine.GET("/ping", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	graph := engine.Group("/graph")
	{
		graph.POST("", noOpHandler)
		graph.GET("/:graphId", func(c *gin.Context) {
			log.Info("Requested graph with Id " + c.Param("graphId"))
			c.Status(http.StatusNotFound)
		})
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
			stage.GET("/:stageId", noOpHandler)
			stage.POST("/:stageId/:operation", stageHandler)
		}
	}

	log.Info("Starting")

	engine.Run()
}
