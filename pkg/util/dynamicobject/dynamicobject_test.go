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

package dynamicobject

import (
	"testing"

	"github.com/megaease/easegress/v2/pkg/util/codectool"
)

func TestDynamicObject(t *testing.T) {
	do := DynamicObject{}
	if do.GetString("name") != "" {
		t.Error("name should be empty")
	}
	do.Set("name", 1)
	if do.GetString("name") != "" {
		t.Error("name should be empty")
	}
	do.Set("name", "obj1")
	if do.GetString("name") != "obj1" {
		t.Error("name should be obj1")
	}
	do.Set("value", 1)
	if do.Get("value") != 1 {
		t.Error("value should be 1")
	}

	do.Set("field1", map[string]interface{}{
		"sub1": 1,
		"sub2": "value2",
	})

	do.Set("field2", []interface{}{"sub1", "sub2"})

	data, err := codectool.MarshalYAML(do)
	if err != nil {
		t.Errorf("yaml.Marshal should succeed: %v", err.Error())
	}

	err = codectool.UnmarshalYAML(data, &do)
	if err != nil {
		t.Errorf("yaml.Marshal should succeed: %v", err.Error())
	}

	if _, ok := do.Get("field1").(map[string]interface{}); !ok {
		t.Errorf("the type of 'field1' should be 'map[string]interface{}'")
	}
}
