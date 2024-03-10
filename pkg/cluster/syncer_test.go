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
	"testing"
	"time"

	"github.com/megaease/easegress/v2/pkg/option"
	"github.com/stretchr/testify/assert"
	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
)

func TestSyncer(t *testing.T) {
	assert := assert.New(t)

	createKeyValue := func(key, value string) *mvccpb.KeyValue {
		return &mvccpb.KeyValue{
			Key:   []byte(key),
			Value: []byte(value),
		}
	}

	// test create syncer fail
	{
		c := &cluster{
			opt: &option.Options{},
		}
		s, err := c.Syncer(time.Second)
		assert.Nil(s)
		assert.NotNil(err)
	}

	// test pull
	{
		// fake client, return error for all operations
		client, err := clientv3.New(clientv3.Config{
			Endpoints: []string{"localhost:0000"},
		})
		assert.Nil(err)
		c := &cluster{
			opt:    &option.Options{},
			client: client,
		}
		s := &syncer{
			cluster:      c,
			client:       client,
			pullInterval: time.Second,
			done:         make(chan struct{}),
		}

		_, err = s.pull("prefix", true)
		assert.NotNil(err)

		_, err = s.pull("key", false)
		assert.NotNil(err)
	}

	// test isDataEqual
	{
		// same length, diff key value
		data1 := map[string]*mvccpb.KeyValue{
			"key1": createKeyValue("key1", "value1"),
		}
		data2 := map[string]*mvccpb.KeyValue{
			"key2": createKeyValue("key2", "value2"),
		}
		assert.False(isDataEqual(data1, data2))

		// same length, same key, diff value
		data1 = map[string]*mvccpb.KeyValue{
			"key1": createKeyValue("key1", "value1"),
		}
		data2 = map[string]*mvccpb.KeyValue{
			"key1": createKeyValue("key1", "value2"),
		}
		assert.False(isDataEqual(data1, data2))

		// same key value
		data1 = map[string]*mvccpb.KeyValue{
			"key1": createKeyValue("key1", "value1"),
		}
		data2 = map[string]*mvccpb.KeyValue{
			"key1": createKeyValue("key1", "value1"),
		}
		assert.True(isDataEqual(data1, data2))
	}
}
