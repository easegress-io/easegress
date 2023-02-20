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

package grpcprot

import (
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/metadata"
	"testing"
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
