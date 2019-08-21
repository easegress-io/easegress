package cluster

import (
	"sync"
)

// Cluster is the open cluster interface.
type Cluster interface {
	Layout() *Layout

	Get(key string) (*string, error)
	GetPrefix(prefix string) (map[string]string, error)

	Put(key, value string) error
	// The lease may be expired or revoked, it's callers' duty to
	// care the situation.
	PutUnderLease(key, value string) error
	PutAndDelete(map[string]*string) error
	PutAndDeleteUnderLease(map[string]*string) error

	Delete(key string) error
	DeletePrefix(prefix string) error

	// Currently we doesn't support to cancel watch.
	Watch(key string) (<-chan *string, error)
	WatchPrefix(prefix string) (<-chan map[string]*string, error)

	Mutex(name string) (Mutex, error)

	CloseServer(wg *sync.WaitGroup)
	StartServer() (chan struct{}, chan struct{}, error)

	Close(wg *sync.WaitGroup)

	PurgeMember(member string) error
}
