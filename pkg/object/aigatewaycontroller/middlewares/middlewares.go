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

package middlewares

import (
	"fmt"
	"reflect"

	"github.com/megaease/easegress/v2/pkg/object/aigatewaycontroller/aicontext"
)

type (
	// MiddlewareSpec defines the specification for middleware in the AI Gateway Controller.
	MiddlewareSpec struct {
		Name          string             `json:"name" jsonschema:"required"`
		Kind          string             `json:"kind" jsonschema:"required"`
		SemanticCache *SemanticCacheSpec `json:"semanticCache,omitempty"`
	}

	// Middleware defines the interface for middleware in the AI Gateway Controller.
	Middleware interface {
		Name() string
		Kind() string
		Spec() *MiddlewareSpec
		Handle(ctx *aicontext.Context)

		init(spec *MiddlewareSpec)
		validate(spec *MiddlewareSpec) error
	}
)

var (
	middlewareTypeRegistry = map[string]reflect.Type{}
)

const (
	semanticCacheMiddlewareKind = "SemanticCache"
)

func NewMiddleware(spec *MiddlewareSpec) Middleware {
	if middlewareType, exists := middlewareTypeRegistry[spec.Kind]; exists {
		middleware := reflect.New(middlewareType).Interface().(Middleware)
		middleware.init(spec)
		return middleware
	}
	return nil
}

func ValidateSpec(spec *MiddlewareSpec) error {
	if spec == nil {
		return fmt.Errorf("middleware spec cannot be nil")
	}
	if middlewareType, exists := middlewareTypeRegistry[spec.Kind]; exists {
		middleware := reflect.New(middlewareType).Interface().(Middleware)
		return middleware.validate(spec)
	}
	return fmt.Errorf("unknown middleware type: %s", spec.Kind)
}
