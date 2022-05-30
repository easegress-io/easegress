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

package httpbuilder

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuilderResponseBody(t *testing.T) {
	assert := assert.New(t)
	r := &response{
		Response: nil,
		rawBody:  []byte("abc"),
	}
	assert.Equal([]byte("abc"), r.RawBody())
	assert.Equal("abc", r.Body())
	_, err := r.JSONBody()
	assert.NotNil(err)

	r = &response{
		Response: nil,
		rawBody:  []byte("123"),
	}
	_, err = r.JSONBody()
	assert.Nil(err)

	r = &response{
		Response: nil,
		rawBody:  []byte("{{{{{}"),
	}
	_, err = r.YAMLBody()
	assert.NotNil(err)

	r = &response{
		Response: nil,
		rawBody:  []byte("123"),
	}
	_, err = r.YAMLBody()
	assert.Nil(err)
}
