package meshcontroller

type (
	// MeshStorage is for describing basic storage requires
	MeshStorage interface {
		Get(key string) (string, error)
		Set(key, val string) error

		AcquireLock(key string, expireSecond int) error
		GetWithPrefix(prefix string) ([]record, error)

		ReleaseLock(key string) error
		WatchKey(key string, notifyChan chan map[string]*string) error
	}

	// MockEtcdClient
	mockEtcdClient struct {
		endpoints []string
	}

	record struct {
		key string
		val string
	}
)

// Get gets
func (mec *mockEtcdClient) Get(key string) (string, error) {
	var (
		val string
		err error
	)
	return val, err
}

// Set writes ETCD according to provided 'key' and 'val'
func (mec *mockEtcdClient) Set(key string, val string) error {
	var err error

	return err
}

// AcquireLock acquires a lock for specified key
func (mec *mockEtcdClient) AcquireLock(key string, expireSecond int) error {
	var err error

	return err
}

// GetPrefix gets matched key-value pairs with provided prefix
func (mec *mockEtcdClient) GetWithPrefix(prefix string) ([]record, error) {
	var (
		records []record
		err     error
	)

	return records, err
}

// Releaselock releases the lock for specified key
func (mec *mockEtcdClient) ReleaseLock(key string) error {
	var err error

	return err
}

func (mec *mockEtcdClient) WatchKey(key string, notifyChan chan map[string]*string) error {
	var err error

	return err
}
