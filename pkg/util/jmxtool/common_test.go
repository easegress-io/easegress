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

package jmxtool

import (
	"reflect"
	"testing"
)

func Test_extractKVs(t *testing.T) {
	// extractKVs for int
	prefix := "test"
	want := []map[string]string{{prefix: "1"}}
	got := extractKVs(prefix, 1)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("error for extract int")
	}

	// extractKVs for interface array
	want = []map[string]string{
		{prefix + ".0": "1"},
		{prefix + ".1": "2"},
		{prefix + ".2": "3"},
	}
	got = extractKVs(prefix, []interface{}{1, 2, 3})
	if !reflect.DeepEqual(got, want) {
		t.Errorf("error for extract []interface")
	}
}
