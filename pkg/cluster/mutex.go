package cluster

import (
	"context"
	"time"

	"go.etcd.io/etcd/clientv3/concurrency"
)

// Mutex is a cluster level mutex.
type Mutex interface {
	Lock() error
	Unlock() error
}

type mutex struct {
	m       *concurrency.Mutex
	timeout time.Duration
}

func (m *mutex) Lock() error {
	ctx, cancel := context.WithTimeout(context.Background(), m.timeout)
	defer cancel()

	return m.m.Lock(ctx)
}

func (m *mutex) Unlock() error {
	ctx, cancel := context.WithTimeout(context.Background(), m.timeout)
	defer cancel()

	return m.m.Unlock(ctx)
}

func (c *cluster) Mutex(name string) (Mutex, error) {
	session, err := c.getSession()
	if err != nil {
		return nil, err
	}

	return &mutex{
		m:       concurrency.NewMutex(session, name),
		timeout: c.requestTimeout,
	}, nil
}
