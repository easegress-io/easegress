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
package common

import (
	"testing"
)

func TestStrings(t *testing.T) {
	str1 := "Hello, World"
	b := S2b(str1)
	str2 := B2s(b)
	if str1 != str2 {
		t.Errorf("expected %s, result %s", str1, str2)
	}

	b = []byte{72, 101, 108, 108, 111, 44, 32, 87, 111, 114, 108, 100}
	s := B2s(b)
	if s != str1 {
		t.Errorf("expected %s, result %s", str1, s)
	}

}
