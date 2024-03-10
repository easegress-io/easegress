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

package readers

import (
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCallbackReader(t *testing.T) {
	assert := assert.New(t)
	cr := NewCallbackReader(io.NopCloser(strings.NewReader("123")))
	bytesRead := 0
	closed := false
	cr.OnAfter(func(total int, p []byte, err error) {
		bytesRead = total
	})
	cr.OnClose(func() {
		closed = true
	})

	data, err := io.ReadAll(cr)
	assert.Nil(err)
	assert.Equal("123", string(data))
	assert.Equal(len([]byte("123")), bytesRead)

	cr.Close()
	assert.True(closed)
}
