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

package vectordb

type (
	// Document represents a document in the vector database.
	Document struct {
		Content string `json:"content"`
		// Metadata can be used to store additional information about the document.
		Metadata map[string]interface{} `json:"metadata,omitempty"`
	}

	// VectorDBHandler is the interface for vector database middleware.
	VectorDBHandler interface {
		// TODO: should be extended params before release?
		SimilaritySearch(query string, options ...Option) ([]Document, error)
		// TODO: should be extended params before release?
		// InsertDocuments inserts documents into the vector database, returning the IDs of the inserted documents or error if any.
		InsertDocuments(docs []Document, options ...Option) ([]string, error)

		Init(spec *VectorDBSpec)
	}

	// VectorDBSpec defines the specification for a vector database middleware.
	VectorDBSpec struct {
		Dimensions int     `json:"dimensions" jsonschema:"required"`
		Threshold  float64 `json:"threshold" jsonschema:"required"`
	}
)

//func (spec *VectorDBSpec) validate() error {
//	if spec.Redis == nil {
//		return fmt.Errorf("vectorDB spec must have a redis spec")
//	}
//	if err := spec.Redis.validate(); err != nil {
//		return fmt.Errorf("vectorDB redis spec validation failed: %w", err)
//	}
//	if spec.Dimensions <= 0 {
//		return fmt.Errorf("vectorDB dimensions must be greater than 0")
//	}
//	if spec.Threshold < 0 || spec.Threshold > 1 {
//		return fmt.Errorf("vectorDB threshold must be between 0 and 1")
//	}
//	return nil
//}

//func newVectorDBHandler(spec *VectorDBSpec) VectorDBHandler {
//	handler := &redisVectorDBHandler{}
//	handler.init(spec)
//	return handler
//}

//type (
//	redisVectorDBHandler struct {
//	}
//)
//
//var _ VectorDBHandler = (*redisVectorDBHandler)(nil)
//
//func (spec *VectorRedisSpec) validate() error {
//	if spec.URL == "" {
//		return fmt.Errorf("redis URL is required for vectorDB")
//	}
//	return nil
//}
//
//func (h *redisVectorDBHandler) init(spec *VectorDBSpec) {
//	// TODO
//}
//
//func (h *redisVectorDBHandler) Get(input []float32) (body []byte, err error) {
//	// TODO
//	return nil, nil
//}
//
//func (h *redisVectorDBHandler) Put(input []float32, body []byte) (err error) {
//	// TODO
//	return nil
//}
