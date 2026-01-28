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

package pgvector

import (
	"slices"

	"github.com/megaease/easegress/v2/pkg/object/aigatewaycontroller/middlewares/vectordb/vecdbtypes"
)

var (
	validDistanceAlgorithms = []string{"<=>", "<->", "<#>", "<+>", "<~>", "<%>"}
)

type (
	// PostgresVectorQuery represents a query for vector data in a PostgreSQL database.
	PostgresVectorQuery struct {
		tableName         string
		vectorKey         string
		vectorValues      []float32
		distanceAlgorithm string
		filters           string
		limit             int
		offset            int
	}

	Option func(query *PostgresVectorQuery)
)

// NewPostgresVectorQuery creates a new PostgresVectorQuery with the specified table name, vector key, and vector values.
func NewPostgresVectorQuery(tableName, vectorKey string, vectorValues []float32, opts ...Option) *PostgresVectorQuery {
	query := &PostgresVectorQuery{
		tableName:    tableName,
		vectorKey:    vectorKey,
		vectorValues: vectorValues,
	}

	for _, opt := range opts {
		opt(query)
	}

	if query.distanceAlgorithm == "" || !slices.Contains(validDistanceAlgorithms, query.distanceAlgorithm) {
		query.distanceAlgorithm = "<=>"
	}

	if query.limit <= 0 {
		query.limit = 1
	}

	return query
}

// WithFilters sets the filters for the PostgresVectorQuery.
func WithFilters(filters string) Option {
	return func(query *PostgresVectorQuery) {
		query.filters = filters
	}
}

// WithDistanceAlgorithm sets the distance algorithm for the PostgresVectorQuery.
func WithDistanceAlgorithm(algorithm string) Option {
	return func(query *PostgresVectorQuery) {
		if slices.Contains(validDistanceAlgorithms, algorithm) {
			query.distanceAlgorithm = algorithm
		} else {
			query.distanceAlgorithm = "<=>"
		}
	}
}

// WithLimit sets the limit for the PostgresVectorQuery.
func WithLimit(limit int) Option {
	return func(query *PostgresVectorQuery) {
		query.limit = limit
	}
}

// WithOffset sets the offset for the PostgresVectorQuery.
func WithOffset(offset int) Option {
	return func(query *PostgresVectorQuery) {
		query.offset = offset
	}
}

func toPostgresQueryOptions(options vecdbtypes.HandlerSearchOptions) ([]Option, error) {
	opts := []Option{
		WithLimit(options.Limit),
		WithOffset(options.Offset),
	}

	if options.PostgresDistanceAlgorithm != "" && slices.Contains(validDistanceAlgorithms, options.PostgresDistanceAlgorithm) {
		opts = append(opts, WithDistanceAlgorithm(options.PostgresDistanceAlgorithm))
	} else {
		opts = append(opts, WithDistanceAlgorithm("<=>"))
	}

	if options.PostgresFilters != "" {
		opts = append(opts, WithFilters(options.PostgresFilters))
	}

	return opts, nil
}
