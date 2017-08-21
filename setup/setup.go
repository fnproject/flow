package setup

import (
	"os"
	"github.com/sirupsen/logrus"
	"strings"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/url"
	"strconv"
	protopersistence "github.com/AsynkronIT/protoactor-go/persistence"
	"github.com/fnproject/completer/persistence"
	"github.com/fnproject/completer/actor"
	"github.com/fnproject/completer/server"
)

const (
	EnvFnApiURL         = "api_url"
	EnvDBURL            = "db_url"
	EnvLogLevel         = "log_level"
	EnvListen           = "listen"
	EnvSnapshotInterval = "snapshot_interval"
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

func InitFromEnv() (*server.Server, error) {

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

	provider, blobStore, err := InitStorageFromEnv()
	if err != nil {
		return nil, err
	}

	graphManager, err := actor.NewGraphManager(provider, blobStore, GetString(EnvFnApiURL))

	if err != nil {
		return nil, err

	}

	srv, err := server.New(graphManager, blobStore, GetString(EnvListen))
	if err != nil {
		return nil, err
	}

	return srv, nil
}

func InitStorageFromEnv() (protopersistence.ProviderState, persistence.BlobStore, error) {
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
		return protopersistence.NewInMemoryProvider(snapshotInterval), persistence.NewInMemBlobStore(), nil
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
