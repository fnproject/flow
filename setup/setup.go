package setup

import (
	"github.com/fnproject/flow/actor"
	"github.com/fnproject/flow/cluster"
	"github.com/fnproject/flow/persistence"
	"github.com/fnproject/flow/server"
	"github.com/fnproject/flow/sharding"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"fmt"
	"github.com/fnproject/flow/blobs"
	"net/url"
	"os"
	"strconv"
	"strings"
)

const (
	envFnAPIURL = "API_URL"
	envDBURL    = "DB_URL"
	envLogLevel = "LOG_LEVEL"
	envListen   = "LISTEN"
	envGrpcListen   = "GRPC_LISTEN"

	envSnapshotInterval = "SNAPSHOT_INTERVAL"
	envRequestTimeout   = "REQUEST_TIMEOUT"

	envClusterNodeCount  = "CLUSTER_NODE_COUNT"
	envClusterShardCount = "CLUSTER_SHARD_COUNT"
	envClusterNodePrefix = "CLUSTER_NODE_PREFIX"
	envClusterNodeID     = "CLUSTER_NODE_ID"
	envClusterNodePort   = "CLUSTER_NODE_PORT"

	envZipkinURL = "ZIPKIN_URL"
)

var log = logrus.New().WithField("logger", "setup")

// InitFromEnv sets up a whole  flow service from env/config
func InitFromEnv() (*server.Server, *server.InternalServer,*blobs.Server, error) {
	cwd, err := os.Getwd()
	if err != nil {
		logrus.WithError(err).Fatalln("")
	}
	// Replace forward slashes in case this is windows, URL parser errors
	cwd = strings.Replace(cwd, "\\", "/", -1)
	// Set viper configuration and activate its reading from env
	setDefaultConfig(envFnAPIURL, "http://localhost:8080/r")
	setDefaultConfig(envDBURL, fmt.Sprintf("sqlite3://%s/data/flow.db", cwd))
	setDefaultConfig(envLogLevel, "debug")
	setDefaultConfig(envListen, fmt.Sprintf(":8081"))
	setDefaultConfig(envGrpcListen, "localhost:9999")
	setDefaultConfig(envSnapshotInterval, "1000")
	setDefaultConfig(envRequestTimeout, "60000ms")

	setDefaultConfig(envClusterNodeCount, "1")
	setDefaultConfig(envClusterShardCount, "1")
	setDefaultConfig(envClusterNodePrefix, "node-")
	setDefaultConfig(envClusterNodeID, "0")
	setDefaultConfig(envClusterNodePort, "19081")

	logLevel, err := logrus.ParseLevel(getStringConfig(envLogLevel))
	if err != nil {
		logrus.WithError(err).Fatalln("Invalid log level.")
	}
	logrus.SetLevel(logLevel)

	gin.SetMode(gin.ReleaseMode)
	if logLevel == logrus.DebugLevel {
		gin.SetMode(gin.DebugMode)
	}

	provider, blobStore, err := initStorageFromEnv()
	if err != nil {
		return nil, nil, nil,err
	}

	nodeCount := getIntConfig(envClusterNodeCount)
	var shardCount int
	if len(getStringConfig(envClusterShardCount)) == 0 {
		shardCount = 10 * nodeCount
	} else {
		shardCount = getIntConfig(envClusterShardCount)
	}
	shardExtractor := sharding.NewFixedSizeExtractor(shardCount)

	clusterSettings := &cluster.Settings{
		NodeCount:  nodeCount,
		NodeID:     getIntConfig(envClusterNodeID),
		NodePrefix: getStringConfig(envClusterNodePrefix),
		NodePort:   getIntConfig(envClusterNodePort),
	}
	clusterManager := cluster.NewManager(clusterSettings, shardExtractor)

	shards := clusterManager.LocalShards()

	localGraphManager, err := actor.NewGraphManager(provider, blobStore, getStringConfig(envFnAPIURL), shardExtractor, shards)
	if err != nil {
		return nil, nil,nil, err
	}
	localServer, err := server.NewInternalFlowService(localGraphManager, ":"+getStringConfig(envClusterNodePort))
	if err != nil {
		return nil, nil,nil, err
	}

	apiServer, err := server.NewAPIServer(clusterManager, getStringConfig(envListen), getStringConfig(envGrpcListen), getStringConfig(envZipkinURL))

	if err != nil {
		return nil, nil,nil, err
	}

	blobServer := blobs.NewFromEngine(blobStore,apiServer.Engine)

	return apiServer, localServer, blobServer,nil
}

func getIntConfig(key string) int {
	stringVal := getStringConfig(key)
	intVal,err := strconv.Atoi(stringVal)
	if err !=nil  {
		panic(fmt.Sprintf("parameter %s with val \"%s\" could not be converted to an int",key,stringVal))
	}
	return intVal

}

func getStringConfig(key string) string {
	val := os.Getenv(key)
	if val !="" {
		return val
	}
	return configDefaults[key]
}


var configDefaults = make(map[string]string)

func setDefaultConfig(key string, val string) {
	configDefaults[key] = val
}

func initStorageFromEnv() (persistence.ProviderState, blobs.Store, error) {
	dbURLString := getStringConfig(envDBURL)
	dbURL, err := url.Parse(dbURLString)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid DB URL in %s : %s", envDBURL, dbURLString)
	}

	snapshotIntervalStr := getStringConfig(envSnapshotInterval)
	snapshotInterval, ok := strconv.Atoi(snapshotIntervalStr)
	if ok != nil {
		snapshotInterval = 1000
	}
	if dbURL.Scheme == "inmem" {
		log.Info("Using in-memory persistence")
		return persistence.NewInMemoryProvider(snapshotInterval), blobs.NewInMemBlobStore(), nil
	}

	dbConn, err := persistence.CreateDBConnection(dbURL)
	if err != nil {
		return nil, nil, err
	}

	log.WithField("driver",dbConn.DriverName()).Info("Creating SQL Event store")
	storageProvider, err := persistence.NewSQLProvider(dbConn, snapshotInterval)
	if err != nil {
		return nil, nil, err
	}

	log.WithField("driver",dbConn.DriverName()).Info("Creating SQL Blob Store")
	blobStore, err := blobs.NewSQLBlobStore(dbConn)

	if err != nil {
		return nil, nil, err
	}

	return storageProvider, blobStore, nil

}
