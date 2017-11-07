package main

import (
	"os"

	_ "github.com/mattn/go-sqlite3"

	"github.com/fnproject/flow/setup"
	"github.com/sirupsen/logrus"
)

var log = logrus.WithField("logger", "main")

func main() {

	server, err := setup.InitFromEnv()
	if err != nil {
		log.WithError(err).Errorf("Failed to set up service")
		os.Exit(1)

	}

	server.Run()

}
