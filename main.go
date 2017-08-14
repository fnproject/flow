package main

import (
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"net/http"
)

func main() {

	engine := gin.Default()

	engine.GET("/ping", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	graph := engine.Group("/graph")
	{
		graph.GET("/:graphId", func(c *gin.Context) {
			log.Info("Requested graph with Id " + c.Param("graphId"))
			c.Status(http.StatusNotFound)
		})
	}

	log.Info("Starting")

	engine.Run()
}
