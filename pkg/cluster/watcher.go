/*
 * Copyright (c) 2017, The Easegress Authors
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
	"fmt"

	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"

	"github.com/megaease/easegress/v2/pkg/logger"
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

func (w *watcher) WatchRaw(key string) (<-chan *clientv3.Event, error) {
	ctx, cancel := context.WithCancel(context.Background())
	watchResp := w.w.Watch(ctx, key)

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

func (w *watcher) WatchRawPrefix(prefix string) (<-chan map[string]*clientv3.Event, error) {
	ctx, cancel := context.WithCancel(context.Background())
	watchResp := w.w.Watch(ctx, prefix, clientv3.WithPrefix())

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

func (w *watcher) WatchWithOp(key string, ops ...ClientOp) (<-chan map[string]*string, error) {
	newOps := []clientv3.OpOption{}
	for _, o := range ops {
		if opOption := getOpOption(o); opOption != nil {
			newOps = append(newOps, opOption)
		}
	}

	// NOTE: Can't use Context with timeout here.
	ctx, cancel := context.WithCancel(context.Background())
	watchResp := w.w.Watch(ctx, key, newOps...)
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
					logger.Errorf("watch %s with ops %v canceled: %v", key, ops, resp.Err())
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
						logger.Errorf("BUG: key %s with ops %v received unknown event type %v",
							key, ops, event.Type)
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
