package cluster

import (
	stdcontext "context"

	"github.com/megaease/easegateway/pkg/logger"

	"go.etcd.io/etcd/clientv3"
	"go.etcd.io/etcd/mvcc/mvccpb"
)

func (c *cluster) Watch(key string) (<-chan *string, error) {
	client, err := c.getClient()
	if err != nil {
		return nil, err
	}

	getResp, err := client.Get(c.requestContext(), key)
	if err != nil {
		return nil, err
	}

	w := make(chan *string, 10)
	if len(getResp.Kvs) != 0 {
		value := string(getResp.Kvs[0].Value)
		w <- &value
	}

	// NOTE: can't use Context with timeout here.
	ctx, cancel := stdcontext.WithCancel(stdcontext.Background())
	watchResp := client.Watch(ctx, key,
		clientv3.WithRev(getResp.Header.Revision+1))

	go func() {
		defer cancel()
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
	client, err := c.getClient()
	if err != nil {
		return nil, err
	}

	getResp, err := client.Get(c.requestContext(), prefix, clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}

	w := make(chan map[string]*string, 10)
	kvs := make(map[string]*string)
	for _, kv := range getResp.Kvs {
		value := string(kv.Value)
		kvs[string(kv.Key)] = &value
	}

	// NOTE: Even kvs is empty, for close firstDone of scheduler
	// in case of there is no config.
	w <- kvs

	// NOTE: can't use Context with timeout here.
	ctx, cancel := stdcontext.WithCancel(stdcontext.Background())
	watchResp := client.Watch(ctx, prefix,
		clientv3.WithRev(getResp.Header.Revision+1),
		clientv3.WithPrefix())

	go func() {
		defer cancel()
		for resp := range watchResp {
			if resp.Canceled {
				logger.Errorf("watch prefix %s canceled: %v", prefix, resp.Err())
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
