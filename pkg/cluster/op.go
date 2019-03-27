package cluster

import (
	"fmt"

	"go.etcd.io/etcd/clientv3"
)

// PutUnderLease stores data which will be eliminated when the lease expired or revoked,
// so the caller should update the data periodically.
func (c *cluster) PutUnderLease(key, value string) error {
	if !c.started {
		return fmt.Errorf("etcd cluster not started yet")
	}

	_, err := c.client.Put(ctx(), key, value,
		clientv3.WithLease(c.session.Lease()))

	return err
}

func (c *cluster) Put(key, value string) error {
	if !c.started {
		return fmt.Errorf("etcd cluster not started yet")
	}

	_, err := c.client.Put(ctx(), key, value)
	return err
}

func (c *cluster) PutAndDeleteUnderLease(kvs map[string]*string) error {
	if !c.started {
		return fmt.Errorf("etcd cluster not started yet")
	}

	return c.putAndDelete(kvs, true)
}

func (c *cluster) PutAndDelete(kvs map[string]*string) error {
	if !c.started {
		return fmt.Errorf("etcd cluster not started yet")
	}

	return c.putAndDelete(kvs, false)
}

func (c *cluster) putAndDelete(kvs map[string]*string, underLease bool) error {
	var ops []clientv3.Op
	for k, v := range kvs {
		if v != nil {
			var opts []clientv3.OpOption
			if underLease {
				opts = append(opts, clientv3.WithLease(c.session.Lease()))
			}
			ops = append(ops, clientv3.OpPut(k, *v, opts...))
		} else {
			ops = append(ops, clientv3.OpDelete(k))
		}
	}
	_, err := c.client.Txn(ctx()).Then(ops...).Commit()
	return err
}

func (c *cluster) Delete(key string) error {
	if !c.started {
		return fmt.Errorf("etcd cluster not started yet")
	}

	_, err := c.client.Delete(ctx(), key)
	return err
}

func (c *cluster) DeletePrefix(prefix string) error {
	if !c.started {
		return fmt.Errorf("etcd cluster not started yet")
	}

	_, err := c.client.Delete(ctx(), prefix, clientv3.WithPrefix())
	return err
}

func (c *cluster) Get(key string) (*string, error) {
	if !c.started {
		return nil, fmt.Errorf("etcd cluster not started yet")
	}

	resp, err := c.client.Get(ctx(), key)
	if err != nil {
		return nil, err
	}

	if len(resp.Kvs) == 0 {
		return nil, nil
	}

	value := string(resp.Kvs[0].Value)
	return &value, nil
}

func (c *cluster) GetPrefix(prefix string) (map[string]string, error) {
	if !c.started {
		return nil, fmt.Errorf("etcd cluster not started yet")
	}

	resp, err := c.client.Get(ctx(), prefix, clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}

	kvs := make(map[string]string)
	for _, kv := range resp.Kvs {
		kvs[string(kv.Key)] = string(kv.Value)
	}

	return kvs, nil
}
