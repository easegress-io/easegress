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

package nginx

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSplitAddressPort(t *testing.T) {
	testCases := []struct {
		listen  string
		address string
		port    int
		err     bool
	}{
		{listen: "127.0.0.1:8000", address: "127.0.0.1", port: 8000, err: false},
		{listen: "127.0.0.1", address: "127.0.0.1", port: 80, err: false},
		{listen: "8000", address: "", port: 8000, err: false},
		{listen: "*:8000", address: "*", port: 8000, err: false},
		{listen: "localhost:8000", address: "localhost", port: 8000, err: false},
		{listen: "[::]:8000", address: "[::]", port: 8000, err: false},
		{listen: "[::1]", address: "[::1]", port: 80, err: false},
		{listen: "", address: "", port: 0, err: true},
		{listen: "[::", address: "", port: 0, err: true},
		{listen: "[::]:", address: "", port: 0, err: true},
		{listen: "[::]:8888", address: "[::]", port: 8888, err: false},
	}
	for _, tc := range testCases {
		msg := fmt.Sprintf("%v", tc)
		address, port, err := splitAddressPort(tc.listen)
		assert.Equal(t, tc.address, address, msg)
		assert.Equal(t, tc.port, port, msg)
		if tc.err {
			assert.NotNil(t, err, msg)
		} else {
			assert.Nil(t, err, msg)
		}
	}
}
