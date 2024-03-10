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
	"google.golang.org/grpc/metadata"
)

func TestValueNotString(t *testing.T) {
	assertion := assert.New(t)
	t.Run("append value type is int", func(t *testing.T) {
		header := NewHeader(metadata.New(nil))
		assertion.Panics(func() {
			header.Add("test", 1)
		})
		assertion.Nil(header.Get("test"))
	})
	t.Run("set value type is int[]", func(t *testing.T) {
		header := NewHeader(metadata.New(nil))
		arr := []int{1, 2, 3}
		assertion.Panics(func() {
			header.Set("test", arr)
		})
	})
}

func TestNewTrailer(t *testing.T) {
	assertion := assert.New(t)
	md := metadata.New(nil)
	trailer := NewTrailer(md)
	assertion.NotNil(trailer)
}

func TestAdd(t *testing.T) {
	assertion := assert.New(t)

	t.Run("append single value", func(t *testing.T) {
		header := NewHeader(metadata.New(nil))
		header.Add("test", "test")
		assertion.Equal([]string{"test"}, header.Get("test"))
	})

	t.Run("append multiple value", func(t *testing.T) {
		header := NewHeader(metadata.New(nil))
		header.Add("test", []string{"test1", "test2"})
		assertion.Equal([]string{"test1", "test2"}, header.Get("test"))
	})

	t.Run("append integer value", func(t *testing.T) {
		header := NewHeader(metadata.New(nil))
		assertion.Panics(func() {
			header.Add("test", 10)
		})
	})

	t.Run("raw add", func(t *testing.T) {
		header := NewHeader(metadata.New(nil))
		header.RawAdd("test", "test1", "test2")
		assertion.Equal([]string{"test1", "test2"}, header.Get("test"))
	})
}

func TestSet(t *testing.T) {
	assertion := assert.New(t)

	t.Run("set single value", func(t *testing.T) {
		header := NewHeader(metadata.New(nil))
		header.Set("test", "test")
		assertion.Equal([]string{"test"}, header.Get("test"))
	})

	t.Run("set multiple value", func(t *testing.T) {
		header := NewHeader(metadata.New(nil))
		header.Set("test", []string{"test1", "test2"})
		assertion.Equal([]string{"test1", "test2"}, header.Get("test"))
	})

	t.Run("set integer value", func(t *testing.T) {
		header := NewHeader(metadata.New(nil))
		assertion.Panics(func() {
			header.Set("test", 10)
		})
	})

	t.Run("raw set", func(t *testing.T) {
		header := NewHeader(metadata.New(nil))
		header.RawSet("test", "test1", "test2")
		assertion.Equal([]string{"test1", "test2"}, header.Get("test"))
	})
}

func TestRawGet(t *testing.T) {
	assertion := assert.New(t)

	t.Run("get multiple value", func(t *testing.T) {
		header := NewHeader(metadata.New(nil))
		header.Set("test", []string{"test1", "test2"})
		assertion.Equal([]string{"test1", "test2"}, header.RawGet("test"))
	})
}

func TestHeaderMerge(t *testing.T) {
	assertion := assert.New(t)

	t.Run("merge header", func(t *testing.T) {
		header1 := NewHeader(metadata.New(nil))
		header1.Set("test", []string{"test1", "test2"})
		header1.Merge(nil)
		assertion.Equal([]string{"test1", "test2"}, header1.Get("test"))

		header2 := NewHeader(metadata.New(nil))
		header2.Set("test", []string{"test3", "test4"})
		header1.Merge(header2)
		assertion.Equal([]string{"test1", "test2", "test3", "test4"}, header1.Get("test"))
	})
}

func TestHeaderWalk(t *testing.T) {
	assertion := assert.New(t)

	h := NewHeader(metadata.New(nil))
	h.Add("test1", "test1")
	h.Add("test2", "test2")
	h.Add("test3", "test3")

	count := 0
	h.Walk(func(key string, value interface{}) bool {
		count++
		return true
	})
	assertion.Equal(3, count)

	count = 0
	h.Walk(func(key string, value interface{}) bool {
		count++
		return false
	})
	assertion.Equal(1, count)
}

func TestHeaderClone(t *testing.T) {
	assertion := assert.New(t)

	t.Run("merge header", func(t *testing.T) {
		header1 := NewHeader(metadata.New(nil))
		header1.Set("test", []string{"test1", "test2"})
		header2 := header1.Clone().(*Header)
		assertion.Equal([]string{"test1", "test2"}, header2.Get("test"))
	})
}
