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
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProtocol(t *testing.T) {
	assert := assert.New(t)

	p := &Protocol{}

	_, err := p.CreateRequest(nil)
	assert.Error(err)

	_, err = p.CreateRequest(NewFakeServerStream(context.Background()))
	assert.NoError(err)

	_, err = p.CreateResponse(nil)
	assert.NoError(err)

	assert.Panics(func() { p.NewRequestInfo() })
	assert.Panics(func() { p.BuildRequest(nil) })
	assert.Panics(func() { p.NewResponseInfo() })
	assert.Panics(func() { p.BuildResponse(nil) })
}
