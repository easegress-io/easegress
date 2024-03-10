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

package grpcproxy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGrpcCodecString(t *testing.T) {
	grpcCodec := GrpcCodec{}
	assert.Equal(t, codecName, grpcCodec.Name())
}

func TestCodecMarshalUnmarshal(t *testing.T) {
	grpcCodec := GrpcCodec{}
	f1 := &frame{payload: []byte("test")}
	data, err := grpcCodec.Marshal(f1)
	assert.NoError(t, err)
	assert.Equal(t, f1.payload, data)

	f2 := &frame{}
	err = grpcCodec.Unmarshal(data, f2)
	assert.NoError(t, err)
	assert.Equal(t, f1.payload, f2.payload)

	m1 := "abcd"
	assert.Panics(t, func() { grpcCodec.Marshal(m1) })
	assert.Panics(t, func() { grpcCodec.Unmarshal(data, m1) })
}
