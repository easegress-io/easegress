package cluster

import (
	"go.etcd.io/etcd/clientv3"
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
	value := string(resp.Kvs[0].Value)

	return &value, nil
}

func (c *cluster) GetPrefix(prefix string) (map[string]string, error) {
	client, err := c.getClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.Get(c.requestContext(), prefix, clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}

	kvs := make(map[string]string)
	for _, kv := range resp.Kvs {
		kvs[string(kv.Key)] = string(kv.Value)
	}

	return kvs, nil
}
