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

// Package urlclusteranalyzer provides url cluster analyzer.
package urlclusteranalyzer

import (
	"strings"
	"sync"

	lru "github.com/hashicorp/golang-lru"
)

const (
	maxValues = 20
	maxLayers = 256
)

// URLClusterAnalyzer is url cluster analyzer.
type URLClusterAnalyzer struct {
	slots []*field
	mutex *sync.Mutex
	cache *lru.Cache
}

type field struct {
	constant        string
	subFields       []*field
	variableField   *field
	isVariableField bool
	pattern         string
}

func newField(name string) *field {
	f := &field{
		constant:        name,
		subFields:       make([]*field, 0),
		variableField:   nil,
		isVariableField: false,
		pattern:         "",
	}

	return f
}

// New creates a URLClusterAnalyzer.
func New() *URLClusterAnalyzer {
	cache, _ := lru.New(4098)
	u := &URLClusterAnalyzer{
		mutex: &sync.Mutex{},
		slots: make([]*field, maxLayers),
		cache: cache,
	}

	for i := 0; i < maxLayers; i++ {
		u.slots[i] = newField("root")
	}
	return u
}

// GetPattern extracts the pattern of a Restful url path.
// A field of the path occurs more than 20 distinct values will be considered as a variables.
// e.g. input: /com/megaease/users/123/friends/456
// output: /com/megaease/users/*/friends/*
func (u *URLClusterAnalyzer) GetPattern(urlPath string) string {
	if urlPath == "" {
		return ""
	}
	if val, ok := u.cache.Get(urlPath); ok {
		return val.(string)
	}

	var values []string
	if urlPath[0] == '/' {
		values = strings.Split(urlPath[1:], "/")
	} else {
		values = strings.Split(urlPath, "/")
	}

	pos := len(values)
	if len(values) >= maxLayers {
		pos = maxLayers - 1
	}
	currField := u.slots[pos]
	currPattern := currField.pattern

	u.mutex.Lock()
	defer u.mutex.Unlock()

LOOP:
	for i := 0; i < len(values); i++ {

		// a known variable field
		if currField.isVariableField {
			currPattern = currField.pattern
			currField = currField.variableField
			continue
		}

		// a known constant field
		for _, v := range currField.subFields {
			if v.constant == values[i] {
				currPattern = v.pattern
				currField = v
				continue LOOP
			}
		}

		newF := newField(values[i])
		currField.subFields = append(currField.subFields, newF)

		// find out a new variable field
		if len(currField.subFields) > maxValues {
			currField.isVariableField = true
			currField.variableField = newField("*")
			currField.pattern = currPattern + "/*"

			currPattern = currField.pattern

			currField = currField.variableField
			continue
		}

		// find out a new constant field
		newF.pattern = currPattern + "/" + values[i]
		currPattern = newF.pattern
		currField = newF
	}

	u.cache.Add(urlPath, currPattern)

	return currPattern
}
