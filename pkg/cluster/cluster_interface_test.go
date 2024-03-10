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

	"github.com/stretchr/testify/assert"
	clientv3 "go.etcd.io/etcd/client/v3"
)

func TestGetOpOption(t *testing.T) {
	tests := []struct {
		wantFunc clientv3.OpOption
		op       ClientOp
	}{
		{clientv3.WithPrefix(), OpPrefix},
		{clientv3.WithFilterPut(), OpNotWatchPut},
		{clientv3.WithFilterDelete(), OpNotWatchDelete},
		{clientv3.WithKeysOnly(), OpKeysOnly},
	}
	for _, tc := range tests {
		op1 := clientv3.Op{}
		op2 := clientv3.Op{}
		tc.wantFunc(&op1)
		getOpOption(tc.op)(&op2)
		assert.Equal(t, op1, op2)
	}

	assert.Nil(t, getOpOption("unknown"))
}
