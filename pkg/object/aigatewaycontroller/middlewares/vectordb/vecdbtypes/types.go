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

package vecdbtypes

import "errors"

var ErrSimilaritySearchNotFound = errors.New("not found a result that matches the query in vector database")

type (
	// Document represents a document in the vector database.
	Document struct {
		Content string `json:"content"`
		// Metadata can be used to store additional information about the document.
		Metadata map[string]interface{} `json:"metadata,omitempty"`
	}

	// VectorDBHandler is the interface for vector database middleware.
	VectorDB interface {
		// // TODO: should be extended params before release?
		// SimilaritySearch(query string, options ...Option) ([]Document, error)
		// // TODO: should be extended params before release?
		// // InsertDocuments inserts documents into the vector database, returning the IDs of the inserted documents or error if any.
		// InsertDocuments(docs []Document, options ...Option) ([]string, error)

		// SimilaritySearch(vec []float32, options ...Option) (string, error)
		// InsertDocuments(vec []float32, doc string, options ...Option) ([]string, error)

		CreateSchema(options ...Option) VectorHandler
	}

	VectorHandler interface {
		SimilaritySearch(vec []float32, options ...Option) (map[string]any, error)
		InsertDocuments(vec []float32, doc map[string]any, options ...Option) ([]string, error)
	}

	// CommonSpec defines the specification for a vector database middleware.
	CommonSpec struct {
		Type           string  `json:"type"`
		Dimensions     int     `json:"dimensions" jsonschema:"required"`
		Threshold      float64 `json:"threshold" jsonschema:"required"`
		CollectionName string  `json:"collectionName" jsonschema:"required"`
	}
)
