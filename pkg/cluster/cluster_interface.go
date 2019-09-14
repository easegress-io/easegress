package cluster

import (
	"sync"
)

type (
	// Cluster is the open cluster interface.
	Cluster interface {
		Layout() *Layout

		Get(key string) (*string, error)
		GetPrefix(prefix string) (map[string]string, error)

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
		Close()
	}
)
