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

package routers

import (
	"net/http"
	"testing"

	"github.com/megaease/easegress/v2/pkg/protocols/httpprot"
	"github.com/stretchr/testify/assert"
)

func TestRegisterAndCreate(t *testing.T) {
	assert := assert.New(t)
	assert.Panics(func() {
		Register(&Kind{})
	})

	Register(&Kind{
		Name: "test",
		CreateInstance: func(rules Rules) Router {
			return nil
		},
	})
	assert.NotNil(kinds["test"])

	assert.Panics(func() {
		Register(&Kind{Name: "test"})
	})

	assert.Nil(Create("test", nil))
	assert.Nil(Create("foo", nil))
}

func TestGetCaptures(t *testing.T) {
	assert := assert.New(t)

	stdr, _ := http.NewRequest(http.MethodGet, "http://www.megaease.com:8080", nil)
	req, _ := httpprot.NewRequest(stdr)

	ctx := NewContext(req)
	res := ctx.GetCaptures()
	assert.Equal(0, len(res))
	assert.NotNil(res)

	ctx = NewContext(req)
	ctx.Params.Keys = []string{"a", "b", "c"}
	ctx.Params.Values = []string{"1", "2", "3"}
	res = ctx.GetCaptures()
	assert.Equal(3, len(res))
	assert.NotNil(res)
	assert.Equal(map[string]string{
		"a": "1",
		"b": "2",
		"c": "3",
	}, res)

	res = ctx.GetCaptures()
	assert.Equal(3, len(res))
	assert.NotNil(res)
	assert.Equal(map[string]string{
		"a": "1",
		"b": "2",
		"c": "3",
	}, res)
}

func TestGetQueries(t *testing.T) {
	assert := assert.New(t)

	stdr, _ := http.NewRequest(http.MethodGet, "http://www.megaease.com:8080/foo?a=1&b=2", nil)
	req, _ := httpprot.NewRequest(stdr)
	ctx := NewContext(req)

	queries := ctx.GetQueries()
	assert.Len(queries, 2)

	queries = ctx.GetQueries()
	assert.Len(queries, 2)
}
