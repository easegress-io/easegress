package meshcontroller

import "fmt"

const (
	opTypeCreate = "create"
	opTypeUpdate = "update"
	opTypeDelete = "delete"
)

var (
	ErrKeyNoExist = fmt.Errorf("key no exist")
)

type (
	// MeshStorage is for describing basic storage APIs.
	MeshStorage interface {
		Get(key string) (string, error)
		Set(key, val string) error
		Delete(key string) error

		AcquireLock(key string, expireSecond int) error
		GetWithPrefix(prefix string) ([]record, error)

		ReleaseLock(key string) error
		WatchWithPrefix(prefix string) (chan storeOpMsg, error)
		WatchKey(key string) (chan storeOpMsg, error)
	}

	// MockEtcdClient mocks ETCD storage operations.
	mockEtcdClient struct {
		endpoints []string
	}

	record struct {
		key string
		val string
	}

	storeOpMsg struct {
		op     string // operation type, "create/update/delete"
		record record
	}
)

// Get gets ETCD one record by provided 'key'.
func (mec *mockEtcdClient) Get(key string) (string, error) {
	var (
		val string
		err error
	)
	return val, err
}

// Set writes ETCD according to provided 'key' and 'val'.
func (mec *mockEtcdClient) Set(key string, val string) error {
	var err error

	return err
}

// Delete deletes ETCD key by provided 'key'.
func (mec *mockEtcdClient) Delete(key string) error {
	var (
		err error
	)
	return err
}

// AcquireLock acquires a lock for specified key.
func (mec *mockEtcdClient) AcquireLock(key string, expireSecond int) error {
	var err error

	return err
}

// GetPrefix gets matched key-value pairs with provided prefix.
func (mec *mockEtcdClient) GetWithPrefix(prefix string) ([]record, error) {
	var (
		records []record
		err     error
	)

	return records, err
}

// Releaselock releases the lock for specified key.
func (mec *mockEtcdClient) ReleaseLock(key string) error {
	var err error

	return err
}

// Watchkey watch one key.
func (mec *mockEtcdClient) WatchKey(key string) (chan storeOpMsg, error) {
	var (
		err   error
		opMsg chan storeOpMsg
	)
	opMsg = make(chan storeOpMsg)

	return opMsg, err
}

// WatchWithPrefix wathc multiple keys with provided prefix.
func (mec *mockEtcdClient) WatchWithPrefix(prefix string) (chan storeOpMsg, error) {
	var (
		err   error
		opMsg chan storeOpMsg
	)
	opMsg = make(chan storeOpMsg)

	return opMsg, err
}
