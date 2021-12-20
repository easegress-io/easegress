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

package supervisor

import (
	"sync"

	"github.com/megaease/easegress/pkg/cluster"
	"github.com/megaease/easegress/pkg/option"
)

// NewMock return a mock supervisor for testing purpose
func NewMock(options *option.Options, cls cluster.Cluster, businessControllers sync.Map,
	systemControllers sync.Map, objectRegistry *ObjectRegistry, watcher *ObjectEntityWatcher,
	firstHandle bool, firstHandleDone chan struct{}, done chan struct{}) *Supervisor {
	return &Supervisor{
		options:             options,
		cls:                 cls,
		businessControllers: businessControllers,
		systemControllers:   systemControllers,
		objectRegistry:      objectRegistry,
		watcher:             watcher,
		firstHandle:         firstHandle,
		firstHandleDone:     firstHandleDone,
		done:                done,
	}
}

// NewDefaultMock return a mock supervisor for testing purpose
func NewDefaultMock() *Supervisor {
	return &Supervisor{}
}
