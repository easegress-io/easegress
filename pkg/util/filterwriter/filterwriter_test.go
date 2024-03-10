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

package filterwriter

import (
	"bytes"
	"testing"
)

type counterWriter struct {
	count int
}

func (cw *counterWriter) Write(p []byte) (int, error) {
	cw.count++
	return len(p), nil
}

func TestFilterWriter(t *testing.T) {
	cw := &counterWriter{}

	fw := New(cw, func(p []byte) bool {
		return !bytes.Contains(p, []byte("filter out"))
	})

	fw.Write([]byte("abcd"))
	if cw.count != 1 {
		t.Error("data should be filtered in.")
	}

	fw.Write([]byte("abcd filter out"))
	if cw.count > 1 {
		t.Error("data should be filtered out.")
	}
}
