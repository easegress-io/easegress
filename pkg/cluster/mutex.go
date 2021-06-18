/*
 * Copyright (c) 2017, MegaEase
 * All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package cluster

import (
	"context"
	"time"

	"go.etcd.io/etcd/client/v3/concurrency"
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
