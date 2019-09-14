package cluster

import (
	"context"
	"fmt"

	"github.com/megaease/easegateway/pkg/logger"

	"go.etcd.io/etcd/clientv3"
	"go.etcd.io/etcd/mvcc/mvccpb"
)

type (
	watcher struct {
		w    clientv3.Watcher
		done chan struct{}
	}
)

func (c *cluster) Watcher() (Watcher, error) {
	client, err := c.getClient()
	if err != nil {
		return nil, fmt.Errorf("get client failed: %v", err)
	}

	w := clientv3.NewWatcher(client)

	return &watcher{
		w:    w,
		done: make(chan struct{}),
	}, nil
}

func (w *watcher) Watch(key string) (<-chan *string, error) {
	// NOTE: Can't use Context with timeout here.
	ctx, cancel := context.WithCancel(context.Background())
	watchResp := w.w.Watch(ctx, key)

	keyChan := make(chan *string, 10)

	go func() {
		defer cancel()
		defer close(keyChan)

		for {
			select {
			case <-w.done:
				return
			case resp := <-watchResp:
				if resp.Canceled {
					logger.Infof("watch key %s canceled: %v", key, resp.Err())
					return
				}
				if resp.IsProgressNotify() {
					continue
				}
				for _, event := range resp.Events {
					switch event.Type {
					case mvccpb.PUT:
						value := string(event.Kv.Value)
						keyChan <- &value
					case mvccpb.DELETE:
						keyChan <- nil
					default:
						logger.Errorf("BUG: key %s received unknown event type %v",
							key, event.Type)
					}
				}
			}
		}
	}()

	return keyChan, nil
}

func (w *watcher) WatchPrefix(prefix string) (<-chan map[string]*string, error) {
	// NOTE: Can't use Context with timeout here.
	ctx, cancel := context.WithCancel(context.Background())
	watchResp := w.w.Watch(ctx, prefix, clientv3.WithPrefix())

	prefixChan := make(chan map[string]*string, 10)

	go func() {
		defer cancel()
		defer close(prefixChan)

		for {
			select {
			case <-w.done:
				return
			case resp := <-watchResp:
				if resp.Canceled {
					logger.Errorf("watch prefix %s canceled: %v", prefix, resp.Err())
					return
				}
				if resp.IsProgressNotify() {
					continue
				}
				for _, event := range resp.Events {
					switch event.Type {
					case mvccpb.PUT:
						value := string(event.Kv.Value)
						prefixChan <- map[string]*string{
							string(event.Kv.Key): &value,
						}
					case mvccpb.DELETE:
						prefixChan <- map[string]*string{
							string(event.Kv.Key): nil,
						}
					default:
						logger.Errorf("BUG: prefix %s received unknown event type %v",
							prefix, event.Type)
					}
				}
			}
		}
	}()

	return prefixChan, nil
}

func (w *watcher) Close() {
	close(w.done)

	err := w.w.Close()
	if err != nil {
		logger.Errorf("close watcher failed: %v", err)
	}
}
