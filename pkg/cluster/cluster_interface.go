package cluster

import (
	"sync"

	"go.etcd.io/etcd/clientv3"
	"go.etcd.io/etcd/mvcc/mvccpb"
)

type (
	// Cluster is the open cluster interface.
	Cluster interface {
		Layout() *Layout

		Get(key string) (*string, error)
		GetPrefix(prefix string) (map[string]string, error)
		GetRaw(key string) (*mvccpb.KeyValue, error)
		GetRawPrefix(prefix string) (map[string]*mvccpb.KeyValue, error)

		Put(key, value string) error
		PutUnderLease(key, value string) error
		PutAndDelete(map[string]*string) error
		PutAndDeleteUnderLease(map[string]*string) error

		Delete(key string) error
		DeletePrefix(prefix string) error

		Watcher() (Watcher, error)

		Mutex(name string) (Mutex, error)

		CloseServer(wg *sync.WaitGroup)
		StartServer() (chan struct{}, chan struct{}, error)

		Close(wg *sync.WaitGroup)

		PurgeMember(member string) error
	}

	// Watcher wraps etcd watcher.
	Watcher interface {
		Watch(key string) (<-chan *string, error)
		WatchPrefix(prefix string) (<-chan map[string]*string, error)
		WatchRaw(key string) (<-chan *clientv3.Event, error)
		WatchRawPrefix(prefix string) (<-chan map[string]*clientv3.Event, error)

		WatchFromRev(key string, rev int64) (<-chan *string, error)
		WatchPrefixFromRev(prefix string, rev int64) (<-chan map[string]*string, error)
		WatchRawPrefixFromRev(prefix string, rev int64) (<-chan map[string]*clientv3.Event, error)
		WatchRawFromRev(key string, rev int64) (<-chan *clientv3.Event, error)

		Close()
	}
)
