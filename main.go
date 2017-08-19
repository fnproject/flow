package main

import (
	"os"

	_ "github.com/mattn/go-sqlite3"

	"github.com/fnproject/completer/server"
	"github.com/fnproject/completer/actor"
	"github.com/sirupsen/logrus"
	"github.com/fnproject/completer/persistence"
	"github.com/fnproject/completer/setup"
)

var log = logrus.WithField("logger", "main")

func main() {

	setup.Init()

	provider, err := persistence.NewProviderFromEnv()
	if err != nil {
		log.WithError(err).Error("Failed to create persistence provider")
		os.Exit(1)
		return
	}

	graphManager, err := actor.NewGraphManagerFromEnv(provider)

	if err != nil {
		log.WithError(err).Error("Failed to create graph manager")
		os.Exit(1)
		return
	}

	srv, err := server.NewFromEnv(graphManager)
	if err != nil {
		log.WithError(err).Error("Failed to start server")
		return
	}

	srv.Run()
}
