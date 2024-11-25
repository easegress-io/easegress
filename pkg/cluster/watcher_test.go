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
	"errors"
	"testing"
	"time"

	"github.com/megaease/easegress/v2/pkg/option"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	pbv3 "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
)

type mockWatcher struct {
	mock.Mock
}

var _ clientv3.Watcher = &mockWatcher{}

func (m *mockWatcher) Watch(ctx context.Context, key string, opts ...clientv3.OpOption) clientv3.WatchChan {
	args := m.Called(ctx, key, opts)
	return args.Get(0).(chan clientv3.WatchResponse)
}

func (m *mockWatcher) RequestProgress(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockWatcher) Close() error {
	args := m.Called()
	return args.Error(0)
}

func TestWatcher(t *testing.T) {
	assert := assert.New(t)

	createCancelResp := func() clientv3.WatchResponse {
		return clientv3.WatchResponse{
			Canceled: true,
		}
	}

	createNotifyResp := func() clientv3.WatchResponse {
		return clientv3.WatchResponse{
			Events:          []*clientv3.Event{},
			Canceled:        false,
			Created:         false,
			CompactRevision: 0,
			Header: pbv3.ResponseHeader{
				Revision: 1,
			},
		}
	}

	createEventResp := func(events []*clientv3.Event) clientv3.WatchResponse {
		return clientv3.WatchResponse{
			Events: events,
		}
	}

	createEvent := func(eventType mvccpb.Event_EventType, key, value string) *clientv3.Event {
		return &clientv3.Event{
			Type: eventType,
			Kv: &mvccpb.KeyValue{
				Key:   []byte(key),
				Value: []byte(value),
			},
		}
	}

	// test fail to create watcher
	{
		c := &cluster{
			opt: &option.Options{},
		}
		w, err := c.Watcher()
		assert.Nil(w)
		assert.NotNil(err)
	}

	// test Close
	{
		m := &mockWatcher{}
		m.On("Close").Return(errors.New("close error"))
		w := &watcher{
			w:    m,
			done: make(chan struct{}),
		}
		w.Close()
		m.AssertExpectations(t)
	}

	// test Watch
	{
		m := &mockWatcher{}
		// prepare mock watcher
		{
			respCh := make(chan clientv3.WatchResponse, 20)

			cancelResp := createCancelResp()
			assert.True(cancelResp.Canceled)
			notifyResp := createNotifyResp()
			assert.True(notifyResp.IsProgressNotify())
			eventResp := createEventResp([]*clientv3.Event{
				createEvent(999, "key", "value"), // invalid event type
				createEvent(mvccpb.DELETE, "key", ""),
				createEvent(mvccpb.PUT, "key", "value"),
			})
			assert.False(eventResp.Canceled)
			assert.False(eventResp.IsProgressNotify())

			m.On("Watch", mock.Anything, mock.Anything, mock.Anything).Return(respCh)
			// notify will continue, event will return nil and "value", canceled will cancel
			respCh <- notifyResp
			respCh <- eventResp
			respCh <- cancelResp
		}

		w := &watcher{
			w:    m,
			done: make(chan struct{}),
		}
		watchCh, err := w.Watch("key")
		assert.Nil(err)

		// output should be nil(for delete), "value"(for put), nil(for cancel)
		select {
		case resp := <-watchCh:
			assert.Nil(resp)
		case <-time.After(time.Second):
			assert.Fail("timeout")
		}

		select {
		case resp := <-watchCh:
			assert.Equal("value", *resp)
		case <-time.After(time.Second):
			assert.Fail("timeout")
		}

		select {
		case resp := <-watchCh:
			assert.Nil(resp)
		case <-time.After(time.Second):
			assert.Fail("timeout")
		}
	}

	// test WatchRaw
	{
		m := &mockWatcher{}
		// prepare mock watcher
		{
			respCh := make(chan clientv3.WatchResponse, 20)

			cancelResp := createCancelResp()
			assert.True(cancelResp.Canceled)
			notifyResp := createNotifyResp()
			assert.True(notifyResp.IsProgressNotify())
			eventResp := createEventResp([]*clientv3.Event{
				createEvent(999, "key", "value"), // invalid event type
				createEvent(mvccpb.DELETE, "key", ""),
				createEvent(mvccpb.PUT, "key", "value"),
			})
			assert.False(eventResp.Canceled)
			assert.False(eventResp.IsProgressNotify())

			m.On("Watch", mock.Anything, mock.Anything, mock.Anything).Return(respCh)
			// notify will continue, event will return nil and "value", canceled will cancel
			respCh <- notifyResp
			respCh <- eventResp
			respCh <- cancelResp
		}

		w := &watcher{
			w:    m,
			done: make(chan struct{}),
		}
		watchCh, err := w.WatchRaw("key")
		assert.Nil(err)

		// output should be nil(for delete), event(for put), nil(for cancel)
		select {
		case resp := <-watchCh:
			assert.Nil(resp)
		case <-time.After(time.Second):
			assert.Fail("timeout")
		}

		select {
		case resp := <-watchCh:
			assert.Equal("value", string(resp.Kv.Value))
		case <-time.After(time.Second):
			assert.Fail("timeout")
		}

		select {
		case resp := <-watchCh:
			assert.Nil(resp)
		case <-time.After(time.Second):
			assert.Fail("timeout")
		}
	}

	// test WatchPrefix
	{
		m := &mockWatcher{}
		// prepare mock watcher
		{
			respCh := make(chan clientv3.WatchResponse, 20)

			cancelResp := createCancelResp()
			assert.True(cancelResp.Canceled)
			notifyResp := createNotifyResp()
			assert.True(notifyResp.IsProgressNotify())
			eventResp := createEventResp([]*clientv3.Event{
				createEvent(999, "key", "value"), // invalid event type
				createEvent(mvccpb.DELETE, "key", ""),
				createEvent(mvccpb.PUT, "key", "value"),
			})
			assert.False(eventResp.Canceled)
			assert.False(eventResp.IsProgressNotify())

			m.On("Watch", mock.Anything, mock.Anything, mock.Anything).Return(respCh)
			// notify will continue, event will return nil and "value", canceled will cancel
			respCh <- notifyResp
			respCh <- eventResp
			respCh <- cancelResp
		}

		w := &watcher{
			w:    m,
			done: make(chan struct{}),
		}
		watchCh, err := w.WatchPrefix("key")
		assert.Nil(err)

		// output should be "key"(for delete), "key":"value"(for put), nil(for cancel)
		select {
		case resp := <-watchCh:
			_, ok := resp["key"]
			assert.True(ok)
		case <-time.After(time.Second):
			assert.Fail("timeout")
		}

		select {
		case resp := <-watchCh:
			value, ok := resp["key"]
			assert.True(ok)
			assert.Equal("value", *value)
		case <-time.After(time.Second):
			assert.Fail("timeout")
		}

		select {
		case resp := <-watchCh:
			assert.Nil(resp)
		case <-time.After(time.Second):
			assert.Fail("timeout")
		}
	}

	// test WatchRawPrefix
	{
		m := &mockWatcher{}
		// prepare mock watcher
		{
			respCh := make(chan clientv3.WatchResponse, 20)

			cancelResp := createCancelResp()
			assert.True(cancelResp.Canceled)
			notifyResp := createNotifyResp()
			assert.True(notifyResp.IsProgressNotify())
			eventResp := createEventResp([]*clientv3.Event{
				createEvent(999, "key", "value"), // invalid event type
				createEvent(mvccpb.DELETE, "key", ""),
				createEvent(mvccpb.PUT, "key", "value"),
			})
			assert.False(eventResp.Canceled)
			assert.False(eventResp.IsProgressNotify())

			m.On("Watch", mock.Anything, mock.Anything, mock.Anything).Return(respCh)
			// notify will continue, event will return nil and "value", canceled will cancel
			respCh <- notifyResp
			respCh <- eventResp
			respCh <- cancelResp
		}

		w := &watcher{
			w:    m,
			done: make(chan struct{}),
		}
		watchCh, err := w.WatchRawPrefix("key")
		assert.Nil(err)

		// output should be "key"(for delete), "key":"value"(for put), nil(for cancel)
		select {
		case resp := <-watchCh:
			_, ok := resp["key"]
			assert.True(ok)
		case <-time.After(time.Second):
			assert.Fail("timeout")
		}

		select {
		case resp := <-watchCh:
			value, ok := resp["key"]
			assert.True(ok)
			assert.Equal("value", string(value.Kv.Value))
		case <-time.After(time.Second):
			assert.Fail("timeout")
		}

		select {
		case resp := <-watchCh:
			assert.Nil(resp)
		case <-time.After(time.Second):
			assert.Fail("timeout")
		}
	}

	// test WatchWithOption
	{
		m := &mockWatcher{}
		// prepare mock watcher
		{
			respCh := make(chan clientv3.WatchResponse, 20)

			cancelResp := createCancelResp()
			assert.True(cancelResp.Canceled)
			notifyResp := createNotifyResp()
			assert.True(notifyResp.IsProgressNotify())
			eventResp := createEventResp([]*clientv3.Event{
				createEvent(999, "key", "value"), // invalid event type
				createEvent(mvccpb.DELETE, "key", ""),
				createEvent(mvccpb.PUT, "key", "value"),
			})
			assert.False(eventResp.Canceled)
			assert.False(eventResp.IsProgressNotify())

			m.On("Watch", mock.Anything, mock.Anything, mock.Anything).Return(respCh)
			// notify will continue, event will return nil and "value", canceled will cancel
			respCh <- notifyResp
			respCh <- eventResp
			respCh <- cancelResp
		}

		w := &watcher{
			w:    m,
			done: make(chan struct{}),
		}
		watchCh, err := w.WatchWithOp("key")
		assert.Nil(err)

		// output should be "key"(for delete), "key":"value"(for put), nil(for cancel)
		select {
		case resp := <-watchCh:
			_, ok := resp["key"]
			assert.True(ok)
		case <-time.After(time.Second):
			assert.Fail("timeout")
		}

		select {
		case resp := <-watchCh:
			value, ok := resp["key"]
			assert.True(ok)
			assert.Equal("value", *value)
		case <-time.After(time.Second):
			assert.Fail("timeout")
		}

		select {
		case resp := <-watchCh:
			assert.Nil(resp)
		case <-time.After(time.Second):
			assert.Fail("timeout")
		}
	}
}
