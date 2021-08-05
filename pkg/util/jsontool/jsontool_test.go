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

package jsontool

import "testing"

func TestJSONTrimNull(t *testing.T) {
	originJSON := `{"a":2,"b":3,"c":null,"d":{"x":"aaa","y":null,"z":"4444"},"e":[1,2,null,4],"f":{},"g":[]}`
	want := `{"a":2,"b":3,"d":{"x":"aaa","z":"4444"},"e":[1,2,4],"f":{},"g":[]}`
	rst, err := TrimNull([]byte(originJSON))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := string(rst)

	if want != got {
		t.Fatalf("want: %s\ngot : %s", want, got)
	}
}

func TestTrimNull(t *testing.T) {
	_, err := TrimNull(nil)

	if err == nil {
		t.Errorf("trim null should be failed")
	}
}
