package setup

import (
	"fmt"
	"github.com/fnproject/flow/actor"
	"github.com/fnproject/flow/persistence"
	"github.com/fnproject/flow/server"
	"github.com/fnproject/flow/sharding"
	"github.com/fnproject/flow/cluster"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"net/url"
	"os"
	"strconv"
	"strings"
	"github.com/spf13/viper"
)

const (
	EnvFnApiURL         = "api_url"
	EnvDBURL            = "db_url"
	EnvLogLevel         = "log_level"
	EnvListen           = "listen"
	EnvSnapshotInterval = "snapshot_interval"
	EnvRequestTimeout   = "request_timeout"

	EnvClusterNodeCount  = "cluster_node_count"
	EnvClusterShardCount = "cluster_shard_count"
	EnvClusterNodePrefix = "cluster_node_prefix"
	EnvClusterNodeID     = "cluster_node_id"
	EnvClusterNodePort   = "cluster_node_port"

	EnvZipkinURL = "zipkin_url"
)

var defaults = make(map[string]string)
var log = logrus.New().WithField("logger", "setup")

func canonKey(key string) string {
	return strings.Replace(strings.Replace(strings.ToLower(key), "-", "_", -1), ".", "_", -1)
}





func InitFromEnv() (*server.Server, error) {

	cwd, err := os.Getwd()
	if err != nil {
		logrus.WithError(err).Fatalln("")
	}
	// Replace forward slashes in case this is windows, URL parser errors
	cwd = strings.Replace(cwd, "\\", "/", -1)
	viper.SetDefault(EnvLogLevel, "debug")
	viper.SetDefault(EnvDBURL, fmt.Sprintf("sqlite3://%s/data/flow.db", cwd))
	viper.SetDefault(EnvListen, fmt.Sprintf(":8081"))
	viper.SetDefault(EnvSnapshotInterval, "1000")
	viper.SetDefault(EnvFnApiURL, "http://localhost:8080/r")
	viper.SetDefault(EnvRequestTimeout, "60000ms")

	// single node defaults
	viper.SetDefault(EnvClusterNodePrefix, "node-")
	viper.SetDefault(EnvClusterNodeID, "0")
	viper.SetDefault(EnvClusterNodeCount, "1")
	viper.SetDefault(EnvClusterNodePort, "8081")

	for _, v := range os.Environ() {
		vals := strings.Split(v, "=")
		defaults[canonKey(vals[0])] = strings.Join(vals[1:], "=")
	}

	logLevel, err := logrus.ParseLevel(viper.GetString(EnvLogLevel))
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

	nodeCount := viper.GetInt(EnvClusterNodeCount)
	var shardCount int
	if len(viper.GetString(EnvClusterShardCount)) == 0 {
		shardCount = 10 * nodeCount
	} else {
		shardCount = viper.GetInt(EnvClusterShardCount)
	}
	shardExtractor := sharding.NewFixedSizeExtractor(shardCount)

	clusterSettings := &cluster.ClusterSettings{
		NodeCount:  nodeCount,
		NodeID:     viper.GetInt(EnvClusterNodeID),
		NodePrefix: viper.GetString(EnvClusterNodePrefix),
		NodePort:   viper.GetInt(EnvClusterNodePort),
	}
	clusterManager := cluster.NewManager(clusterSettings, shardExtractor)

	shards := clusterManager.LocalShards()
	graphManager, err := actor.NewGraphManager(provider, blobStore, viper.GetString(EnvFnApiURL), shardExtractor, shards)
	if err != nil {
		return nil, err
	}

	srv, err := server.New(clusterManager, graphManager, blobStore, viper.GetString(EnvListen), viper.GetDuration(EnvRequestTimeout),viper.GetString(EnvZipkinURL))
	if err != nil {
		return nil, err
	}

	return srv, nil
}

func InitStorageFromEnv() (persistence.ProviderState, persistence.BlobStore, error) {
	dbUrlString := viper.GetString(EnvDBURL)
	dbUrl, err := url.Parse(dbUrlString)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid DB URL in %s : %s", EnvDBURL, dbUrlString)
	}

	snapshotIntervalStr := viper.GetString(EnvSnapshotInterval)
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
