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

package qdrant

type ErrCreateQdrantClient struct {
	Message string
	Err     error
}

func NewErrCreateQdrantClient(message string, err error) *ErrCreateQdrantClient {
	return &ErrCreateQdrantClient{
		Message: message,
		Err:     err,
	}
}

func (e *ErrCreateQdrantClient) Error() string {
	return e.Message + ": " + e.Err.Error()
}

type ErrCreateQdrantCollection struct {
	Message string
	Err     error
}

func NewErrCreateQdrantCollection(message string, err error) *ErrCreateQdrantCollection {
	return &ErrCreateQdrantCollection{
		Message: message,
		Err:     err,
	}
}

func (e *ErrCreateQdrantCollection) Error() string {
	return e.Message + ": " + e.Err.Error()
}

type ErrUnexpectedCollectionSchema struct {
	Message string
	Err     error
}

func NewErrUnexpectedCollectionSchema(message string, err error) *ErrUnexpectedCollectionSchema {
	return &ErrUnexpectedCollectionSchema{
		Message: message,
		Err:     err,
	}
}

func (e *ErrUnexpectedCollectionSchema) Error() string {
	return e.Message + ": " + e.Err.Error()
}

type ErrInsertDocuments struct {
	Message string
	Err     error
}

func NewErrInsertDocuments(message string, err error) *ErrInsertDocuments {
	return &ErrInsertDocuments{
		Message: message,
		Err:     err,
	}
}

func (e *ErrInsertDocuments) Error() string {
	return e.Message + ": " + e.Err.Error()
}

type ErrSimilaritySearch struct {
	Message string
	Err     error
}

func NewErrSimilaritySearch(message string, err error) *ErrSimilaritySearch {
	return &ErrSimilaritySearch{
		Message: message,
		Err:     err,
	}
}

func (e *ErrSimilaritySearch) Error() string {
	return e.Message + ": " + e.Err.Error()
}
