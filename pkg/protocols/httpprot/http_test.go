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

package httpprot

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHeader(t *testing.T) {
	assert := assert.New(t)
	req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:8080", strings.NewReader("body string"))
	assert.Nil(err)

	multiEqual := func(want interface{}, got []interface{}) {
		for _, g := range got {
			assert.Equal(want, g)
		}
	}

	header := newHeader(req.Header)
	header.Add("X-Users", "abc123")
	header.Add("X-Users", "def123")
	multiEqual([]string{"abc123", "def123"}, []interface{}{header.Values("X-Users"), req.Header.Values("X-Users")})
	multiEqual("abc123", []interface{}{header.Get("X-Users"), req.Header.Get("X-Users")})

	header.Set("X-Users", "qwe123")
	multiEqual([]string{"qwe123"}, []interface{}{header.Values("X-Users"), req.Header.Values("X-Users")})
	multiEqual("qwe123", []interface{}{header.Get("X-Users"), req.Header.Get("X-Users")})

	header.Del("X-Users")
	multiEqual([]string(nil), []interface{}{header.Values("X-Users"), req.Header.Values("X-Users")})
	multiEqual("", []interface{}{header.Get("X-Users"), req.Header.Get("X-Users")})

	header2 := header.Clone()
	header2.Add("X-Users", "header2")
	multiEqual([]string(nil), []interface{}{header.Values("X-Users"), req.Header.Values("X-Users")})
	multiEqual("", []interface{}{header.Get("X-Users"), req.Header.Get("X-Users")})
	assert.Equal("header2", header2.Get("X-Users"))

	header.Add("X-User", "abc")
	header.Add("X-Device", "phone")
	res := map[string][]string{}
	header.Walk(func(key string, value interface{}) bool {
		res[key] = value.([]string)
		return true
	})
	assert.Equal(map[string][]string{"X-User": {"abc"}, "X-Device": {"phone"}}, res)
}

/*
func TestPayload(t *testing.T) {
	assert := assert.New(t)

	reader := strings.NewReader("body string")
	payload := newPayload(reader)

	wg := sync.WaitGroup{}
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			newReader := payload.NewReader()
			data, err := io.ReadAll(newReader)
			assert.Nil(err)
			assert.Equal("body string", string(data))
		}()
	}
	wg.Wait()

	payload.SetReader(strings.NewReader("new body"), true)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			newReader := payload.NewReader()
			data, err := io.ReadAll(newReader)
			assert.Nil(err)
			assert.Equal("new body", string(data))
		}()
	}
	wg.Wait()

	// close only close original reader, the buffer not changed.
	// so in current implementation, after close payload, we still
	// can read data from reader
	newReader := payload.NewReader()
	payload.Close()
	data, err := io.ReadAll(newReader)
	assert.Nil(err)
	assert.Equal("new body", string(data))
}

*/
