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
	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
)

// PutUnderLease stores data under lease.
// The lifecycle of lease is the same with the member,
// it will be revoked after purging the member.
func (c *cluster) PutUnderLease(key, value string) error {
	client, err := c.getClient()
	if err != nil {
		return err
	}

	lease, err := c.getLease()
	if err != nil {
		return err
	}

	_, err = client.Put(c.requestContext(), key, value, clientv3.WithLease(lease))

	return err
}

func (c *cluster) Put(key, value string) error {
	client, err := c.getClient()
	if err != nil {
		return err
	}

	_, err = client.Put(c.requestContext(), key, value)

	return err
}

func (c *cluster) PutAndDeleteUnderLease(kvs map[string]*string) error {
	return c.putAndDelete(kvs, true)
}

func (c *cluster) PutAndDelete(kvs map[string]*string) error {
	return c.putAndDelete(kvs, false)
}

func (c *cluster) putAndDelete(kvs map[string]*string, underLease bool) error {
	client, err := c.getClient()
	if err != nil {
		return err
	}

	lease, err := c.getLease()
	if err != nil {
		return err
	}

	var ops []clientv3.Op
	for k, v := range kvs {
		if v != nil {
			var opts []clientv3.OpOption
			if underLease {
				opts = append(opts, clientv3.WithLease(lease))
			}
			ops = append(ops, clientv3.OpPut(k, *v, opts...))
		} else {
			ops = append(ops, clientv3.OpDelete(k))
		}
	}

	_, err = client.Txn(c.requestContext()).Then(ops...).Commit()

	return err
}

func (c *cluster) Delete(key string) error {
	client, err := c.getClient()
	if err != nil {
		return err
	}

	_, err = client.Delete(c.requestContext(), key)

	return err
}

func (c *cluster) DeletePrefix(prefix string) error {
	client, err := c.getClient()
	if err != nil {
		return err
	}

	_, err = client.Delete(c.requestContext(), prefix, clientv3.WithPrefix())

	return err
}

func (c *cluster) Get(key string) (*string, error) {
	kv, err := c.GetRaw(key)
	if err != nil || kv == nil {
		return nil, err
	}

	value := string(kv.Value)

	return &value, nil
}

func (c *cluster) GetRaw(key string) (*mvccpb.KeyValue, error) {
	client, err := c.getClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.Get(c.requestContext(), key)
	if err != nil {
		return nil, err
	}

	if len(resp.Kvs) == 0 {
		return nil, nil
	}

	return resp.Kvs[0], nil
}

func (c *cluster) GetPrefix(prefix string) (map[string]string, error) {
	kvs := make(map[string]string)
	rawKVs, err := c.GetRawPrefix(prefix)
	if err != nil {
		return kvs, err
	}

	for _, kv := range rawKVs {
		kvs[string(kv.Key)] = string(kv.Value)
	}

	return kvs, nil
}

func (c *cluster) GetRawPrefix(prefix string) (map[string]*mvccpb.KeyValue, error) {
	kvs := make(map[string]*mvccpb.KeyValue)

	client, err := c.getClient()
	if err != nil {
		return kvs, err
	}

	resp, err := client.Get(c.requestContext(), prefix, clientv3.WithPrefix())
	if err != nil {
		return kvs, err
	}

	for _, kv := range resp.Kvs {
		kvs[string(kv.Key)] = kv
	}

	return kvs, nil
}

func (c *cluster) STM(apply func(concurrency.STM) error) error {
	client, err := c.getClient()
	if err != nil {
		return err
	}
	_, err = concurrency.NewSTM(client, apply)
	return err
}
