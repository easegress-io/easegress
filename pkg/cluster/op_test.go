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
	"testing"
	"time"

	"github.com/megaease/easegress/v2/pkg/option"
	"github.com/stretchr/testify/assert"
	clientv3 "go.etcd.io/etcd/client/v3"
)

func TestOperationFail(t *testing.T) {
	assert := assert.New(t)

	clusterWithoutClient := func() *cluster {
		return &cluster{
			opt: &option.Options{},
		}
	}

	clusterWithClient := func() *cluster {
		return &cluster{
			opt:    &option.Options{},
			client: clientv3.NewCtxClient(context.Background()),
		}
	}

	// cluster without client
	{
		c := clusterWithoutClient()
		_, err := c.getClient()
		assert.NotNil(err)

		err = c.PutUnderLease("test", "test")
		assert.NotNil(err)

		err = c.Put("test", "test")
		assert.NotNil(err)

		err = c.PutAndDelete(map[string]*string{"key": nil})
		assert.NotNil(err)

		err = c.Delete("test")
		assert.NotNil(err)

		err = c.DeletePrefix("test")
		assert.NotNil(err)

		_, err = c.GetRaw("test")
		assert.NotNil(err)

		_, err = c.GetPrefix("test")
		assert.NotNil(err)

		_, err = c.GetRawPrefix("test")
		assert.NotNil(err)

		_, err = c.GetWithOp("test")
		assert.NotNil(err)

		err = c.STM(nil)
		assert.NotNil(err)

		err = c.PutUnderTimeout("test", "test", time.Second)
		assert.NotNil(err)
	}

	{
		c := clusterWithClient()
		_, err := c.getClient()
		assert.Nil(err)
		_, err = c.getLease()
		assert.NotNil(err)

		err = c.PutUnderLease("test", "test")
		assert.NotNil(err)

		err = c.PutAndDelete(map[string]*string{"key": nil})
		assert.NotNil(err)
	}
}
