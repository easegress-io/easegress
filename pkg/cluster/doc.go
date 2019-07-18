// Package cluster contains:
// 1. an etcd server
// 2. an etcd client
//
// The cluster only proivide interface to maintain the etcd members(join/drop), but not data access.
// The client is delegated to the api layer to access data from/to the cluster.
package cluster
