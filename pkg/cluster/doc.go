// A cletcd server  wrapper.
//
// A cluster contains:
//
// 1. a etcd server
//
// 2. a etcd client
//
// The cluster only proivide interface to maintain the etcd members(join/drop), but not data access.
// The client is delegated to the api layer to access data from/to the cluster.
package cluster
