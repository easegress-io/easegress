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

// Package api implements the HTTP API of Easegress.
package api

import (
	"fmt"
	"strings"

	"github.com/megaease/easegress/v2/pkg/object/trafficcontroller"
	"github.com/megaease/easegress/v2/pkg/supervisor"
)

func init() {
	// to avoid import cycle
	RegisterObject(&APIResource{
		Category: trafficcontroller.Category,
		Kind:     trafficcontroller.Kind,
		Name:     strings.ToLower(trafficcontroller.Kind),
		Aliases:  []string{"trafficcontroller", "tc"},
	})
}

const DefaultNamespace = "default"

type (
	APIResource struct {
		Category string
		Kind     string
		Name     string
		Aliases  []string

		// ValiateHook is optional, if set, will be called before create/update/delete object.
		// If it returns an error, the operation will be rejected.
		ValiateHook ValidateHookFunc `json:"-"`
	}

	ValidateHookFunc func(operationType OperationType, spec *supervisor.Spec) error

	OperationType string
)

const (
	OperationTypeCreate OperationType = "create"
	OperationTypeUpdate OperationType = "update"
	OperationTypeDelete OperationType = "delete"
)

// key is Kind name, now only contains api resource of object.
var objectAPIResource = map[string]*APIResource{}

var objectValidateHooks = []ValidateHookFunc{}

func RegisterObject(r *APIResource) {
	if r.Kind == "" {
		panic(fmt.Errorf("%v: empty kind", r))
	}
	if r.Name == "" {
		panic(fmt.Errorf("%v: empty name", r))
	}

	existedObject, existed := objectAPIResource[r.Kind]
	if existed {
		panic(fmt.Errorf("%v and %v got same kind: %s", r, existedObject, r.Kind))
	}
	objectAPIResource[r.Kind] = r

	if r.ValiateHook != nil {
		objectValidateHooks = append(objectValidateHooks, r.ValiateHook)
	}
}

func ObjectAPIResources() []*APIResource {
	resources := []*APIResource{}
	for _, r := range objectAPIResource {
		resources = append(resources, r)
	}
	return resources
}
