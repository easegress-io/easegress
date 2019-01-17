package cluster

import (
	"context"
	"time"

	"go.etcd.io/etcd/clientv3/concurrency"
)

const (
	// mutex config
	mutexTTL = 10 // Second
)

type Mutex interface {
	Lock() error
	Unlock() error
}

type mutex struct {
	m       *concurrency.Mutex
	timeout time.Duration
}

func (m *mutex) Lock() error {
	ctx, _ := context.WithTimeout(ctx(), m.timeout)
	return m.m.Lock(ctx)
}

func (m *mutex) Unlock() error {
	return m.m.Unlock(ctx())
}

func (c *cluster) Mutex(name string, timeout time.Duration) Mutex {
	m := &mutex{timeout: timeout}
	m.m = concurrency.NewMutex(c.session, name)

	return m
}
