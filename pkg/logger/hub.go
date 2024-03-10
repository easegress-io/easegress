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

package logger

import (
	"sync"

	"go.uber.org/zap"
)

var lh *logHub

func init() {
	lh = &logHub{
		loggers: make(map[string]*zap.SugaredLogger),
		mu:      &sync.RWMutex{},
	}
}

type logHub struct {
	loggers map[string]*zap.SugaredLogger

	mu *sync.RWMutex
}

// register registers a logger with name.
func (lh *logHub) register(name string, logger *zap.SugaredLogger) {
	lh.mu.Lock()
	defer lh.mu.Unlock()

	lh.loggers[name] = logger
}

// sync syncs all loggers.
func (lh *logHub) sync() {
	lh.mu.RLock()
	defer lh.mu.RUnlock()

	for _, logger := range lh.loggers {
		logger.Sync()
	}
}
