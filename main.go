package main

import (
	"os"
	"github.com/fnproject/completer/server"
	"github.com/fnproject/completer/actor"
	"github.com/sirupsen/logrus"
)

var log = logrus.WithField("logger", "main")

func main() {
	fnHost := os.Getenv("FN_HOST")
	if fnHost == "" {
		fnHost = "localhost"
	}
	var fnPort = os.Getenv("FN_PORT")
	if fnPort == "" {
		fnPort = "8080"
	}
	listenHost := os.Getenv("COMPLETER_HOST")
	var listenPort = os.Getenv("COMPLETER_PORT")
	if listenPort == "" {
		listenPort = "8081"
	}
    graphManager := actor.NewGraphManager(fnHost, fnPort)

	srv, err := server.NewServer(listenHost, listenPort, graphManager)
	if err != nil {
		log.WithError(err).Error("Failed to start server")
		return
	}
	srv.Run()
}
