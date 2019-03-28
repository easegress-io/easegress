package cluster

import (
	"sync"
	"time"
)

const (
	// Keep alive hearbeat interval in seconds
	KEEP_ALIVE_INTERVAL = 5 * time.Second
)

type MemberStatus struct {
	// the name as an etcd member, only writer has a name
	Name string `yaml:name`
	// the id as an etcd member, only writer has an id
	Id uint64 `yaml:id`

	// writer | reader
	Role string `yaml:role`

	// leader | follower | subscriber | offline
	EtcdStatus string `yaml:etcd_status`

	// keepalive timestamp
	KeepaliveTime int64 `yaml:keepalive_time`
}

type Cluster interface {
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

	Mutex(name string, timeout time.Duration) Mutex

	Leader() string
	Close(wg *sync.WaitGroup)

	Started() bool

	PurgeMember(member string) error

	MemberStatus() MemberStatus
}
