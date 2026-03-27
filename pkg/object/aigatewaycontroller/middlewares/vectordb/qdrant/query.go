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

type QdrantVectorQuery struct {
	collectionName string
	vectorValues   []float32
	limit          uint64
	scoreThreshold float32
	offset         uint64
}

type Option func(*QdrantVectorQuery)

func NewQdrantVectorQuery(collectionName string, vectorValues []float32, opts ...Option) *QdrantVectorQuery {
	query := &QdrantVectorQuery{
		collectionName: collectionName,
		vectorValues:   vectorValues,
	}

	for _, opt := range opts {
		opt(query)
	}

	if query.limit <= 0 {
		query.limit = 1
	}

	return query
}

func WithLimit(limit uint64) Option {
	return func(query *QdrantVectorQuery) {
		query.limit = limit
	}
}

func WithOffset(offset uint64) Option {
	return func(query *QdrantVectorQuery) {
		query.offset = offset
	}
}

func WithScoreThreshold(threshold float32) Option {
	return func(query *QdrantVectorQuery) {
		query.scoreThreshold = threshold
	}
}
