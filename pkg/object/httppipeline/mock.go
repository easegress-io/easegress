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

package httppipeline

import "github.com/megaease/easegress/pkg/supervisor"

// MockFilterSpec help to create FilterSpec for test
func MockFilterSpec(super *supervisor.Supervisor, yamlConfig string, meta *FilterMetaSpec, filterSpec interface{}) *FilterSpec {
	return &FilterSpec{
		super:      super,
		yamlConfig: yamlConfig,
		meta:       meta,
		filterSpec: filterSpec,
	}
}
