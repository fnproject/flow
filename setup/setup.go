package setup

import (
	"github.com/fnproject/flow/actor"
	"github.com/fnproject/flow/persistence"
	"github.com/fnproject/flow/server"
	"github.com/fnproject/flow/sharding"
	"github.com/fnproject/flow/cluster"

	"github.com/sirupsen/logrus"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"

	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
)

const (
	envFnAPIURL         = "api_url"
	envDBURL            = "db_url"
	envLogLevel         = "log_level"
	envListen           = "listen"
	envSnapshotInterval = "snapshot_interval"
	envRequestTimeout   = "request_timeout"

	envClusterNodeCount  = "cluster_node_count"
	envClusterShardCount = "cluster_shard_count"
	envClusterNodePrefix = "cluster_node_prefix"
	envClusterNodeID     = "cluster_node_id"
	envClusterNodePort   = "cluster_node_port"

	envZipkinURL 		 = "zipkin_url"
)

var defaults = make(map[string]string)
var log = logrus.New().WithField("logger", "setup")

func canonKey(key string) string {
	return strings.Replace(strings.Replace(strings.ToLower(key), "-", "_", -1), ".", "_", -1)
}





// InitFromEnv sets up a whole  flow service from env/config
func InitFromEnv() (*server.Server, error) {

	cwd, err := os.Getwd()
	if err != nil {
		logrus.WithError(err).Fatalln("")
	}
	// Replace forward slashes in case this is windows, URL parser errors
	cwd = strings.Replace(cwd, "\\", "/", -1)
	viper.SetDefault(envLogLevel, "debug")
	viper.SetDefault(envDBURL, fmt.Sprintf("sqlite3://%s/data/flow.db", cwd))
	viper.SetDefault(envListen, fmt.Sprintf(":8081"))
	viper.SetDefault(envSnapshotInterval, "1000")
	viper.SetDefault(envFnAPIURL, "http://localhost:8080/r")
	viper.SetDefault(envRequestTimeout, "60000ms")

	// single node defaults
	viper.SetDefault(envClusterNodePrefix, "node-")
	viper.SetDefault(envClusterNodeID, "0")
	viper.SetDefault(envClusterNodeCount, "1")
	viper.SetDefault(envClusterNodePort, "8081")

	for _, v := range os.Environ() {
		vals := strings.Split(v, "=")
		defaults[canonKey(vals[0])] = strings.Join(vals[1:], "=")
	}

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
		return nil, err
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
	graphManager, err := actor.NewGraphManager(provider, blobStore, viper.GetString(envFnAPIURL), shardExtractor, shards)
	if err != nil {
		return nil, err
	}

	srv, err := server.New(clusterManager, graphManager, blobStore, viper.GetString(envListen), viper.GetDuration(envRequestTimeout),viper.GetString(envZipkinURL))
	if err != nil {
		return nil, err
	}

	return srv, nil
}

func initStorageFromEnv() (persistence.ProviderState, persistence.BlobStore, error) {
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
		return persistence.NewInMemoryProvider(snapshotInterval), persistence.NewInMemBlobStore(), nil
	}

	dbConn, err := persistence.CreateDBConnecection(dbURL)
	if err != nil {
		return nil, nil, err
	}

	storageProvider, err := persistence.NewSQLProvider(dbConn, snapshotInterval)
	if err != nil {
		return nil, nil, err
	}

	blobStore, err := persistence.NewSQLBlobStore(dbConn)

	if err != nil {
		return nil, nil, err
	}

	return storageProvider, blobStore, nil

}
