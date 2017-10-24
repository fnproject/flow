package setup

import (
	"fmt"
	"github.com/fnproject/flow/actor"
	"github.com/fnproject/flow/persistence"
	"github.com/fnproject/flow/server"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	EnvFnApiURL         = "api_url"
	EnvDBURL            = "db_url"
	EnvLogLevel         = "log_level"
	EnvListen           = "listen"
	EnvSnapshotInterval = "snapshot_interval"
	EnvRequestTimeout   = "request_timeout"
)

var defaults = make(map[string]string)
var log = logrus.New().WithField("logger", "setup")

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

func GetDurationMs(key string) time.Duration {
	key = canonKey(key)

	strVal := defaults[key]
	val, err := strconv.ParseUint(strVal, 10, 64)
	if err != nil {
		panic(fmt.Sprintf("Invalid value '%s' for config key '%s' - couldn't parse as int", strVal, key))
	}
	return time.Millisecond * time.Duration(val)
}

func InitFromEnv() (*server.Server, error) {

	cwd, err := os.Getwd()
	if err != nil {
		logrus.WithError(err).Fatalln("")
	}
	// Replace forward slashes in case this is windows, URL parser errors
	cwd = strings.Replace(cwd, "\\", "/", -1)
	SetDefault(EnvLogLevel, "debug")
	SetDefault(EnvDBURL, fmt.Sprintf("sqlite3://%s/data/flow.db", cwd))
	SetDefault(EnvListen, fmt.Sprintf(":8081"))
	SetDefault(EnvSnapshotInterval, "1000")
	SetDefault(EnvFnApiURL, "http://localhost:8080/r")
	SetDefault(EnvRequestTimeout, "60000")
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

	provider, blobStore, err := InitStorageFromEnv()
	if err != nil {
		return nil, err
	}

	graphManager, err := actor.NewGraphManager(provider, blobStore, GetString(EnvFnApiURL))

	if err != nil {
		return nil, err

	}

	srv, err := server.New(graphManager, blobStore, GetString(EnvListen), GetDurationMs(EnvRequestTimeout))
	if err != nil {
		return nil, err
	}

	return srv, nil
}

func InitStorageFromEnv() (persistence.ProviderState, persistence.BlobStore, error) {
	dbUrlString := GetString(EnvDBURL)
	dbUrl, err := url.Parse(dbUrlString)
	if err != nil {
		return nil, nil, fmt.Errorf("Invalid DB URL in %s : %s", EnvDBURL, dbUrlString)
	}

	snapshotIntervalStr := GetString(EnvSnapshotInterval)
	snapshotInterval, ok := strconv.Atoi(snapshotIntervalStr)
	if ok != nil {
		snapshotInterval = 1000
	}
	if dbUrl.Scheme == "inmem" {
		log.Info("Using in-memory persistence")
		return persistence.NewInMemoryProvider(snapshotInterval), persistence.NewInMemBlobStore(), nil
	}

	dbConn, err := persistence.CreateDBConnecection(dbUrl)
	if err != nil {
		return nil, nil, err
	}

	storageProvider, err := persistence.NewSqlProvider(dbConn, snapshotInterval)
	if err != nil {
		return nil, nil, err
	}

	blobStore, err := persistence.NewSqlBlobStore(dbConn)

	if err != nil {
		return nil, nil, err
	}

	return storageProvider, blobStore, nil

}
