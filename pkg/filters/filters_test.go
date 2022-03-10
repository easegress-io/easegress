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

package filters

import (
	"testing"

	"github.com/megaease/easegress/pkg/util/yamltool"
)

func TestSpecInherit(t *testing.T) {
	type DerivedSpec struct {
		BaseSpec `yaml:",inline"`
		Field1   string `yaml:"field1"`
	}

	text := []byte(`
kind: Kind1
name: Name1
field1: abc
`)

	var spec Spec
	derived := &DerivedSpec{}
	spec = derived
	yamltool.Unmarshal(text, spec)

	baseSpec := spec.baseSpec()
	baseSpec.pipeline = "pipeline1"

	if derived.Kind() != "Kind1" {
		t.Error("wrong kind")
	}

	if derived.Name() != "Name1" {
		t.Error("wrong name")
	}

	if derived.Field1 != "abc" {
		t.Error("wrong field")
	}

	if derived.Pipeline() != "pipeline1" {
		t.Error("wrong pipeline")
	}
}
