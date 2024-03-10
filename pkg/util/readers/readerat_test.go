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
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReaderAt(t *testing.T) {
	assert := assert.New(t)

	r := bytes.NewReader([]byte("abcdefghijklmnopqrstuvwxyz"))
	ra := NewReaderAt(r)

	r1 := NewReaderAtReader(ra, 0)

	buf := make([]byte, 10)
	n, err := r1.Read(buf)
	assert.Nil(err)
	assert.Equal(10, n)

	n, err = r1.Read(buf)
	assert.Nil(err)
	assert.Equal(10, n)

	n, err = r1.Read(buf)
	assert.Nil(err)
	assert.Equal(6, n)

	n, err = r1.Read(buf)
	assert.Equal(io.EOF, err)
	assert.Equal(0, n)

	r1 = NewReaderAtReader(ra, 7)
	n, err = r1.Read(buf)
	assert.Nil(err)
	assert.Equal(10, n)

	n, err = r1.Read(buf)
	assert.Equal(io.EOF, err)
	assert.Equal(9, n)

	n, err = r1.Read(buf)
	assert.Equal(io.EOF, err)
	assert.Equal(0, n)
	ra.Close()

	ra = NewReaderAt(nil)
	assert.Nil(ra.Close())

	ra = NewReaderAt(io.NopCloser(strings.NewReader("123")))
	assert.Nil(ra.Close())
}
