package setup

import (
	"github.com/fnproject/flow/actor"
	"github.com/fnproject/flow/cluster"
	"github.com/fnproject/flow/persistence"
	"github.com/fnproject/flow/server"
	"github.com/fnproject/flow/sharding"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"

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
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.SetDefault(envFnAPIURL, "http://localhost:8080/r")
	viper.SetDefault(envDBURL, fmt.Sprintf("sqlite3://%s/data/flow.db", cwd))
	viper.SetDefault(envLogLevel, "debug")
	viper.SetDefault(envListen, fmt.Sprintf(":8081"))

	viper.SetDefault(envSnapshotInterval, "1000")
	viper.SetDefault(envRequestTimeout, "60000ms")
	viper.SetDefault(envClusterNodeCount, "1")
	viper.SetDefault(envClusterShardCount, "1")
	viper.SetDefault(envClusterNodePrefix, "node-")
	viper.SetDefault(envClusterNodeID, "0")
	viper.SetDefault(envClusterNodePort, "19081")
	viper.AutomaticEnv()

	logLevel, err := logrus.ParseLevel(viper.GetString(envLogLevel))
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

	nodeCount := viper.GetInt(envClusterNodeCount)
	var shardCount int
	if len(viper.GetString(envClusterShardCount)) == 0 {
		shardCount = 10 * nodeCount
	} else {
		shardCount = viper.GetInt(envClusterShardCount)
	}
	shardExtractor := sharding.NewFixedSizeExtractor(shardCount)

	clusterSettings := &cluster.Settings{
		NodeCount:  nodeCount,
		NodeID:     viper.GetInt(envClusterNodeID),
		NodePrefix: viper.GetString(envClusterNodePrefix),
		NodePort:   viper.GetInt(envClusterNodePort),
	}
	clusterManager := cluster.NewManager(clusterSettings, shardExtractor)

	shards := clusterManager.LocalShards()

	localGraphManager, err := actor.NewGraphManager(provider, blobStore, viper.GetString(envFnAPIURL), shardExtractor, shards)
	if err != nil {
		return nil, nil,nil, err
	}
	localServer, err := server.NewInternalFlowService(localGraphManager, ":"+viper.GetString(envClusterNodePort))
	if err != nil {
		return nil, nil,nil, err
	}

	apiServer, err := server.NewAPIServer(clusterManager, viper.GetString(envListen), viper.GetString(envZipkinURL))

	if err != nil {
		return nil, nil,nil, err
	}

	blobServer := blobs.NewFromEngine(blobStore,apiServer.Engine)

	return apiServer, localServer, blobServer,nil
}

func initStorageFromEnv() (persistence.ProviderState, blobs.Store, error) {
	dbURLString := viper.GetString(envDBURL)
	dbURL, err := url.Parse(dbURLString)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid DB URL in %s : %s", envDBURL, dbURLString)
	}

	snapshotIntervalStr := viper.GetString(envSnapshotInterval)
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

	storageProvider, err := persistence.NewSQLProvider(dbConn, snapshotInterval)
	if err != nil {
		return nil, nil, err
	}

	blobStore, err := blobs.NewSQLBlobStore(dbConn)

	if err != nil {
		return nil, nil, err
	}

	return storageProvider, blobStore, nil

}
