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

package spec

import "testing"

func TestServiceSelector(t *testing.T) {
	selector := &ServiceSelector{
		MatchServices: []string{"order", "delivery"},
		MatchInstanceLabels: map[string]string{
			"release": "canary",
		},
	}

	if !selector.MatchInstance("order", map[string]string{"release": "canary"}) {
		t.Fatalf("not match")
	}

	if selector.MatchInstance("order", map[string]string{"release": "blue"}) {
		t.Fatalf("not match")
	}

	if !selector.MatchInstance("delivery", map[string]string{"release": "canary"}) {
		t.Fatalf("not match")
	}

	if selector.MatchInstance("xxx", map[string]string{"release": "canary"}) {
		t.Fatalf("not match")
	}
}
