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

package grpcprot

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestResponse(t *testing.T) {
	assert := assert.New(t)

	resp := NewResponse()
	assert.NotNil(resp)
	assert.True(resp.IsStream())
	assert.Panics(func() { resp.SetPayload(nil) })
	assert.Panics(func() { resp.GetPayload() })
	assert.Panics(func() { resp.RawPayload() })
	assert.Panics(func() { resp.PayloadSize() })
	assert.Panics(func() { resp.ToBuilderResponse("") })
	assert.NotPanics(func() { resp.Close() })

	header := NewHeader(metadata.New(nil))
	header.Add("test", "test")

	resp.SetHeader(header)
	assert.Equal([]string{"test"}, resp.Header().Get("test"))
	assert.Equal([]string{"test"}, resp.RawHeader().Get("test"))

	resp.SetTrailer(header)
	assert.Equal([]string{"test"}, resp.Trailer().Get("test"))
	assert.Equal([]string{"test"}, resp.RawTrailer().Get("test"))

	assert.Equal(int(codes.OK), resp.StatusCode())

	resp.SetStatus(nil)
	assert.Equal("OK", resp.GetStatus().Message())
	assert.Equal(codes.OK, resp.GetStatus().Code())
	assert.Equal(int(codes.OK), resp.StatusCode())

	resp.SetStatus(status.New(codes.Canceled, "canceled"))
	assert.Equal("canceled", resp.GetStatus().Message())
	assert.Equal(codes.Canceled, resp.GetStatus().Code())
	assert.Equal(int(codes.Canceled), resp.StatusCode())
}
