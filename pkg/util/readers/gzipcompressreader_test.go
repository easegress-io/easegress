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

func TestGzipCompressDecompressReader(t *testing.T) {
	assert := assert.New(t)

	var str string
	for i := 0; i < 200; i++ {
		str += "123123123124234asdjflasjflasfjlaksnvalknfaslkfnalkfnaslfjasfasfasfas"
	}
	compressReader := NewGZipCompressReader(strings.NewReader(str))
	data, err := io.ReadAll(compressReader)
	assert.Nil(err)
	err = compressReader.Close()
	assert.Nil(err)

	assert.NotEqual(str, string(data))
	assert.Less(10*len(string(data)), len(str))

	decompressReader, err := NewGZipDecompressReader(bytes.NewReader(data))
	assert.Nil(err)
	data, err = io.ReadAll(decompressReader)
	assert.Nil(err)
	assert.Equal(str, string(data))
	err = decompressReader.Close()
	assert.Nil(err)
}
