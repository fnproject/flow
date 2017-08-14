package main

import (
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"net/http"
)

func ping(w http.ResponseWriter, r *http.Request) {
	return
}

func main() {

	r := gin.Default()
	r.GET("/ping", func(c *gin.Context) {
		c.Data(200, "text/plain", nil)
	})

	log.Info("Starting")

	r.Run()
}
