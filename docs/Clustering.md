# Fn Flow Clustering Overview

## Running in a Cluster

The simplest way to run the completer in clustered mode is using docker-compose. To run a two node cluster:

```
cd infra
docker-compose up
```

Change the following environment variables to configure your cluster:

- `cluster_node_count`: the total number of nodes in the cluster. This value is used to assign a node to a shard. In order to safely change this value, you must first stop the cluster.
- `cluster_node_id`: the index of the current node in the cluster. Note that evaluating `${cluster_node_prefix}${cluster_node_id}` must return the DNS-resolvable name of this node.

Additional properties supported:

- `cluster_node_prefix`: the prefix of the DNS-resolvable node name (without the node index)
- `cluster_shard_count`: defaults to 10 * `cluster_node_count`. Note that changing this value will result in new mappings of shards to nodes, and previously persisted graph information will no longer be accessible.


## Clustering Design

The self-contained nature of graph processing (no cross-graph computation is allowed) makes the completer ideally suited for sharding at the graph level. Incoming HTTP requests can be inspected for a graph ID, from which we can compute a shard and ultimately the node where they should be processed. 

Due to the at-most guarantees in graph computations which may have side-effects, we must guarantee that a shard is only owned by at most a single node in the cluster at any given time. By using hash-based partitioning to map graph IDs to shards, and then shards to nodes, we can ensure that all requests pertaining to the same graph are processed by the same node in the cluster. 

Upon receiving an HTTP request, the local completer node first determines whether to process locally or forward the HTTP request to another node. The forwarding logic is implemented inside an [HTTP middleware](https://github.com/gin-gonic/gin). Note that some requests have no associated graph ID, e.g. ping, are thus not cluster-aware and are always processed locally. An exception to this are requests to create new graphs (for which no graph ID exists). In this case, a special interceptor will generate and assign a new graph ID to the request, prior to processing by the forwarding interceptor.

At node startup, a supervisor actor is spawned for each shard owned by the local node, as determined by the shard-to-node hash-based partitioning. Each shard supervisor will then spawn any graph actor children that were assigned to that shard and are still active. Note that this is a static strategy, so that changing the number of shards will result in new shard-to-node mappings and any previous graph data will no longer be accessible. It is still possible to modify the number of nodes in the cluster. Doing so requires all nodes to be stopped to ensure consistent allocation of shards to nodes, thus guaranteeing a single actor instance per graph across the cluster.