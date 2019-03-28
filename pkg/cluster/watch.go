package cluster

import (
	"github.com/megaease/easegateway/pkg/logger"

	"go.etcd.io/etcd/clientv3"
	"go.etcd.io/etcd/mvcc/mvccpb"
)

func (c *cluster) Watch(key string) (<-chan *string, error) {
	getResp, err := c.client.Get(newCtx(), key)
	if err != nil {
		return nil, err
	}

	w := make(chan *string, 10)
	if len(getResp.Kvs) != 0 {
		value := string(getResp.Kvs[0].Value)
		w <- &value
	}

	watchResp := c.client.Watch(newCtx(), key,
		clientv3.WithRev(getResp.Header.Revision+1))

	go func() {
		for resp := range watchResp {
			if resp.Canceled {
				logger.Infof("watch key %s canceled", key)
				break
			}
			if resp.IsProgressNotify() {
				continue
			}
			for _, event := range resp.Events {
				switch event.Type {
				case mvccpb.PUT:
					value := string(event.Kv.Value)
					w <- &value
				case mvccpb.DELETE:
					w <- nil
				default:
					logger.Errorf("BUG: key %s received unknown event type %v",
						key, event.Type)
				}
			}
		}

		close(w)
	}()

	return w, nil
}

func (c *cluster) WatchPrefix(prefix string) (<-chan map[string]*string, error) {
	getResp, err := c.client.Get(newCtx(), prefix, clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}

	w := make(chan map[string]*string, 10)
	kvs := make(map[string]*string)
	for _, kv := range getResp.Kvs {
		value := string(kv.Value)
		kvs[string(kv.Key)] = &value
	}
	if len(kvs) != 0 {
		w <- kvs
	}

	watchResp := c.client.Watch(newCtx(), prefix,
		clientv3.WithRev(getResp.Header.Revision+1),
		clientv3.WithPrefix())

	go func() {
		for resp := range watchResp {
			if resp.Canceled {
				logger.Infof("watch prefix %s canceled", prefix)
				break
			}
			if resp.IsProgressNotify() {
				continue
			}
			for _, event := range resp.Events {
				switch event.Type {
				case mvccpb.PUT:
					value := string(event.Kv.Value)
					w <- map[string]*string{
						string(event.Kv.Key): &value,
					}
				case mvccpb.DELETE:
					w <- map[string]*string{
						string(event.Kv.Key): nil,
					}
				default:
					logger.Errorf("BUG: prefix %s received unknown event type %v",
						prefix, event.Type)
				}
			}
		}

		close(w)
	}()

	return w, nil
}
