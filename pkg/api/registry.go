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

// Package api implements the HTTP API of Easegress.
package api

import (
	"fmt"
)

type APIResource struct {
	Kind    string
	Name    string
	Aliases []string
}

// key is Kind name, now only contains api resource of object.
var objectApiResource = map[string]*APIResource{}

func RegisterObject(r *APIResource) {
	if r.Kind == "" {
		panic(fmt.Errorf("%v: empty kind", r))
	}
	if r.Name == "" {
		panic(fmt.Errorf("%v: empty name", r))
	}

	existedObject, existed := objectApiResource[r.Kind]
	if existed {
		panic(fmt.Errorf("%v and %v got same kind: %s", r, existedObject, r.Kind))
	}
	objectApiResource[r.Kind] = r
}

func ObjectApiResources() []*APIResource {
	resources := []*APIResource{}
	for _, r := range objectApiResource {
		resources = append(resources, r)
	}
	return resources
}
