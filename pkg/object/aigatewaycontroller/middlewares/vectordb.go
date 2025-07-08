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

import "fmt"

type (
	// VectorDBHandler is the interface for vector database middleware.
	VectorDBHandler interface {
		Get(input []float32) (body []byte, err error)
		Put(input []float32, body []byte) (err error)

		init(spec *VectorDBSpec)
	}

	// VectorDBSpec defines the specification for a vector database middleware.
	VectorDBSpec struct {
		Redis      *VectorRedisSpec `json:"redis"`
		Dimensions int              `json:"dimensions" jsonschema:"required"`
		Threshold  float64          `json:"threshold" jsonschema:"required"`
	}

	// VectorRedisSpec defines the specification for a Redis vector database.
	VectorRedisSpec struct {
		URL string `json:"url" jsonschema:"required"`
	}
)

func (spec *VectorDBSpec) validate() error {
	if spec.Redis == nil {
		return fmt.Errorf("vectorDB spec must have a redis spec")
	}
	if err := spec.Redis.validate(); err != nil {
		return fmt.Errorf("vectorDB redis spec validation failed: %w", err)
	}
	if spec.Dimensions <= 0 {
		return fmt.Errorf("vectorDB dimensions must be greater than 0")
	}
	if spec.Threshold < 0 || spec.Threshold > 1 {
		return fmt.Errorf("vectorDB threshold must be between 0 and 1")
	}
	return nil
}

func newVectorDBHandler(spec *VectorDBSpec) VectorDBHandler {
	handler := &redisVectorDBHandler{}
	handler.init(spec)
	return handler
}

type (
	redisVectorDBHandler struct {
	}
)

var _ VectorDBHandler = (*redisVectorDBHandler)(nil)

func (spec *VectorRedisSpec) validate() error {
	if spec.URL == "" {
		return fmt.Errorf("redis URL is required for vectorDB")
	}
	return nil
}

func (h *redisVectorDBHandler) init(spec *VectorDBSpec) {
	// TODO
}

func (h *redisVectorDBHandler) Get(input []float32) (body []byte, err error) {
	// TODO
	return nil, nil
}

func (h *redisVectorDBHandler) Put(input []float32, body []byte) (err error) {
	// TODO
	return nil
}
