package setup

import (
	"os"
	"github.com/sirupsen/logrus"
	"strings"
	"fmt"
	"github.com/gin-gonic/gin"
)

const (
	EnvFnApiURL         = "api_url"
	EnvDBURL            = "db_url"
	EnvLogLevel         = "log_level"
	EnvListen           = "listen"
	EnvSnapshotInterval = "snapshot_interval"
)

var defaults = make(map[string]string)

func canonKey(key string) string {
	return strings.Replace(strings.Replace(strings.ToLower(key), "-", "_", -1), ".", "_", -1)
}

func SetDefault(key string, value string) {
	defaults[canonKey(key)] = value
}

func GetString(key string) string {
	key = canonKey(key)
	return defaults[key]
}



func Init() {

	cwd, err := os.Getwd()
	if err != nil {
		logrus.WithError(err).Fatalln("")
	}
	// Replace forward slashes in case this is windows, URL parser errors
	cwd = strings.Replace(cwd, "\\", "/", -1)
	SetDefault(EnvLogLevel, "debug")
	SetDefault(EnvDBURL, fmt.Sprintf("sqlite3://%s/data/completer.db", cwd))
	SetDefault(EnvListen, fmt.Sprintf(":8081"))
	SetDefault(EnvSnapshotInterval, "1000")
	SetDefault(EnvFnApiURL, "http://localhost:8080/r")
	for _, v := range os.Environ() {
		vals := strings.Split(v, "=")
		defaults[canonKey(vals[0])] = strings.Join(vals[1:], "=")
	}

	logLevel, err := logrus.ParseLevel(GetString(EnvLogLevel))
	if err != nil {
		logrus.WithError(err).Fatalln("Invalid log level.")
	}
	logrus.SetLevel(logLevel)

	gin.SetMode(gin.ReleaseMode)
	if logLevel == logrus.DebugLevel {
		gin.SetMode(gin.DebugMode)
	}
}
