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

	"github.com/megaease/easegress/pkg/logger"
	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// Syncer syncs data from ETCD, it uses an ETCD watcher to receive update.
// The syncer keeps a full copy of data, and keeps apply changes onto it when an
// update event is received from the watcher, and then send out the full data copy.
// The syncer also pulls full data from ETCD at a configurable pull interval, this
// is to ensure data consitency, as ETCD watcher may be cancelled if it cannot catch
// up with the key-value store.
type Syncer struct {
	cluster      *cluster
	client       *clientv3.Client
	pullInterval time.Duration
	done         chan struct{}
}

func (c *cluster) Syncer(pullInterval time.Duration) (*Syncer, error) {
	client, err := c.getClient()
	if err != nil {
		return nil, err
	}
	return &Syncer{
		cluster:      c,
		client:       client,
		pullInterval: pullInterval,
		done:         make(chan struct{}),
	}, nil
}

func (s *Syncer) pull(key string, prefix bool) (map[string]*mvccpb.KeyValue, error) {
	if prefix {
		result, err := s.cluster.GetRawPrefix(key)
		if err != nil {
			logger.Errorf("failed to pull data for prefix %s: %v", key, err)
		}
		return result, err
	}

	kv, err := s.cluster.GetRaw(key)
	if err != nil {
		logger.Errorf("failed to pull data for key %s: %v", key, err)
		return nil, err
	}

	result := make(map[string]*mvccpb.KeyValue)
	if kv != nil {
		result[string(kv.Key)] = kv
	}

	return result, nil
}

func (s *Syncer) watch(key string, prefix bool) (clientv3.Watcher, clientv3.WatchChan) {
	opts := make([]clientv3.OpOption, 0, 1)
	if prefix {
		opts = append(opts, clientv3.WithPrefix())
	}
	watcher := clientv3.NewWatcher(s.client)
	watchChan := watcher.Watch(context.Background(), key, opts...)
	logger.Debugf("watcher created for key %s", key)
	return watcher, watchChan
}

func isKeyValueChanged(oldKV, newKV *mvccpb.KeyValue) bool {
	if oldKV == nil {
		return true
	}
	if newKV.ModRevision <= oldKV.ModRevision {
		return false
	}
	return string(newKV.Value) != string(oldKV.Value)
}

func (s *Syncer) run(key string, prefix bool, send func(data map[string]*mvccpb.KeyValue)) {
	watcher, watchChan := s.watch(key, prefix)
	defer watcher.Close()

	// pull and send out full data copy. otherwise, if the application retrieves a copy
	// of data at the beginnng, and if data is updated after the retrieval and before the
	// syncer starts, it may take a long time (the pull interval) for the application to
	// receive latest data.
	data, err := s.pull(key, prefix)
	if err == nil {
		send(data)
	} else {
		data = make(map[string]*mvccpb.KeyValue)
	}

	ticker := time.NewTicker(s.pullInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.done:
			return

		case <-ticker.C:
			newData, err := s.pull(key, prefix)
			if err != nil {
				continue
			}
			// always send data in this case even there're no changes to ensure data
			// consistency
			data = newData

		case resp := <-watchChan:
			if resp.Canceled {
				// ETCD cancel a watcher when it cannot catch up with the progress of
				// the key-value store. And no matter what happens, we need to restart
				// the watcher.
				logger.Debugf("watch key %s canceled: %v", key, resp.Err())
				watcher.Close()
				watcher, watchChan = s.watch(key, prefix)
				continue
			}
			if resp.IsProgressNotify() {
				continue
			}

			// apply changes to existing data
			changed := false
			for _, event := range resp.Events {
				k := string(event.Kv.Key)
				oldKV, newKV := data[k], event.Kv
				switch event.Type {
				case mvccpb.PUT:
					if isKeyValueChanged(oldKV, newKV) {
						data[k] = newKV
						changed = true
					}
				case mvccpb.DELETE:
					if oldKV != nil && newKV.ModRevision > oldKV.ModRevision {
						delete(data, k)
						changed = true
					}
				default:
					logger.Errorf("BUG: key %s received unknown event type %v", k, event.Type)
				}
			}
			if !changed {
				continue
			}
		}

		send(data)
	}
}

func (s *Syncer) Sync(key string) (<-chan *string, error) {
	ch := make(chan *string, 10)

	fn := func(data map[string]*mvccpb.KeyValue) {
		if kv := data[key]; kv == nil {
			ch <- nil
		} else {
			value := string(kv.Value)
			ch <- &value
		}
	}

	go func() {
		defer close(ch)
		s.run(key, false, fn)
	}()

	return ch, nil
}

func (s *Syncer) SyncRaw(key string) (<-chan *mvccpb.KeyValue, error) {
	ch := make(chan *mvccpb.KeyValue, 10)

	fn := func(data map[string]*mvccpb.KeyValue) {
		ch <- data[key]
	}

	go func() {
		defer close(ch)
		s.run(key, false, fn)
	}()

	return ch, nil
}

func (s *Syncer) SyncPrefix(prefix string) (<-chan map[string]string, error) {
	ch := make(chan map[string]string, 10)

	fn := func(data map[string]*mvccpb.KeyValue) {
		m := make(map[string]string, len(data))
		for k, v := range data {
			m[k] = string(v.Value)
		}
		ch <- m
	}

	go func() {
		defer close(ch)
		s.run(prefix, true, fn)
	}()

	return ch, nil
}

func (s *Syncer) SyncRawPrefix(prefix string) (<-chan map[string]*mvccpb.KeyValue, error) {
	ch := make(chan map[string]*mvccpb.KeyValue, 10)

	fn := func(data map[string]*mvccpb.KeyValue) {
		// make a copy of data as it may be modified after the function returns
		m := make(map[string]*mvccpb.KeyValue, len(data))
		for k, v := range data {
			m[k] = v
		}
		ch <- m
	}

	go func() {
		defer close(ch)
		s.run(prefix, true, fn)
	}()

	return ch, nil
}

func (s *Syncer) Close() {
	close(s.done)
}
