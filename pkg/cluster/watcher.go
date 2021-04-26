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

func (w *watcher) WatchFromRev(key string, rev int64) (<-chan *string, error) {
	// NOTE: Can't use Context with timeout here.
	ctx, cancel := context.WithCancel(context.Background())
	watchResp := w.w.Watch(ctx, key, clientv3.WithRev(rev))

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

func (w *watcher) Watch(key string) (<-chan *string, error) {
	return w.WatchFromRev(key, 0)
}

func (w *watcher) WatchRawFromRev(key string, rev int64) (<-chan *clientv3.Event, error) {
	ctx, cancel := context.WithCancel(context.Background())
	watchResp := w.w.Watch(ctx, key, clientv3.WithRev(rev))

	eventChan := make(chan *clientv3.Event, 10)

	go func() {
		defer cancel()
		defer close(eventChan)

		for {
			select {
			case <-w.done:
				return
			case resp := <-watchResp:
				if resp.Canceled {
					logger.Infof("watch raw key %s canceled: %v", key, resp.Err())
					return
				}
				if resp.IsProgressNotify() {
					continue
				}
				for idx, event := range resp.Events {
					switch event.Type {
					case mvccpb.PUT:
						eventChan <- resp.Events[idx]
					case mvccpb.DELETE:
						eventChan <- nil
					default:
						logger.Errorf("BUG: key %s received unknown event type %v",
							key, event.Type)
					}
				}
			}
		}
	}()

	return eventChan, nil
}

func (w *watcher) WatchRaw(key string) (<-chan *clientv3.Event, error) {
	return w.WatchRawFromRev(key, 0)
}

func (w *watcher) WatchPrefixFromRev(prefix string, rev int64) (<-chan map[string]*string, error) {
	// NOTE: Can't use Context with timeout here.
	ctx, cancel := context.WithCancel(context.Background())
	watchResp := w.w.Watch(ctx, prefix, clientv3.WithPrefix(), clientv3.WithRev(rev))

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

func (w *watcher) WatchPrefix(prefix string) (<-chan map[string]*string, error) {
	return w.WatchPrefixFromRev(prefix, 0)
}

func (w *watcher) WatchRawPrefixFromRev(prefix string, rev int64) (<-chan map[string]*clientv3.Event, error) {
	ctx, cancel := context.WithCancel(context.Background())
	watchResp := w.w.Watch(ctx, prefix, clientv3.WithPrefix(), clientv3.WithRev(rev))

	prefixChan := make(chan map[string]*clientv3.Event, 10)

	go func() {
		defer cancel()
		defer close(prefixChan)

		for {
			select {
			case <-w.done:
				return
			case resp := <-watchResp:
				if resp.Canceled {
					logger.Errorf("watch raw prefix %s canceled: %v", prefix, resp.Err())
					return
				}
				if resp.IsProgressNotify() {
					continue
				}
				for idx, event := range resp.Events {
					switch event.Type {
					case mvccpb.PUT:
						prefixChan <- map[string]*clientv3.Event{
							string(event.Kv.Key): resp.Events[idx],
						}
					case mvccpb.DELETE:
						prefixChan <- map[string]*clientv3.Event{
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

func (w *watcher) WatchRawPrefix(prefix string) (<-chan map[string]*clientv3.Event, error) {
	return w.WatchRawPrefixFromRev(prefix, 0)
}

func (w *watcher) Close() {
	close(w.done)

	err := w.w.Close()
	if err != nil {
		logger.Errorf("close watcher failed: %v", err)
	}
}
