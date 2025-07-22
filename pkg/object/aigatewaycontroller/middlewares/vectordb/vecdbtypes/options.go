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

type Option func(*Options)

type Schema interface {
	SchemaType() string
}

// Options is the struct for options.
type Options struct {
	// DBName is the name of the vector database.
	DBName string
	// Schema is the schema for the vector database.
	Schema Schema
}

type HandlerInsertOption func(*HandlerInsertOptions)

type HandlerInsertOptions struct {
	// RedisPrefix is the prefix for Redis vector database.
	RedisPrefix string
}

// WithRedisPrefix returns a HandlerInsertOption for setting the Redis prefix.
func WithRedisPrefix(prefix string) HandlerInsertOption {
	return func(opts *HandlerInsertOptions) {
		opts.RedisPrefix = prefix
	}
}

type HandlerSearchOption func(*HandlerSearchOptions)

type HandlerSearchOptions struct {
	// Limit is the maximum number of results to return.
	Limit int
	// Offset is the number of results to skip.
	Offset int
	// SortBy is the field to sort the results by.
	SortBy []string
	// Timeout is the maximum time to wait for the search to complete.
	Timeout int
	// ScoreThreshold is the minimum score for a result to be included.
	ScoreThreshold float32
	// SelectedFields is the fields to return in the results.
	SelectedFields []string

	// RedisFilters is the filters conditions for Redis vector database.
	RedisFilters string
	// RedisVectorFilterKey is the key for the vector filter in Redis.
	RedisVectorFilterKey string
	// RedisVectorFilterValues is the value for the vector filter in Redis.
	RedisVectorFilterValues []float32
	// RedisNoContent is a flag to indicate whether to return only the IDs of the results without the content.
	RedisNoContent bool
	// RedisVerbatim is a flag to indicate whether to return the verbatim content of the results.
	RedisVerbatim bool
	// RedisWithScores is a flag to indicate whether to return the score of the results.
	RedisWithScores bool
	// RedisWithSortKeys is a flag to indicate whether to return the sort keys of the results.
	RedisWithSortKeys bool
	// RedisInKeys limits the result to a given set of keys specified in the list.
	RedisInKeys []string
	// RedisInFields limits the result to a given set of fields specified in the list.
	RedisInFields []string

	// PostgresVectorFilterKey is the key for the vector filter in Postgres.
	PostgresVectorFilterKey string
	// PostgresVectorFilterValues is the value for the vector filter in Postgres.
	PostgresVectorFilterValues []float32
	// PostgresDistanceAlgorithm is the distance algorithm to use for the vector filter in Postgres.
	PostgresDistanceAlgorithm string
	// PostgresFilters is the filters conditions for Postgres vector database.
	PostgresFilters string
}

// WithLimit returns a HandlerSearchOption for setting the limit on the number of results.
func WithLimit(limit int) HandlerSearchOption {
	return func(opts *HandlerSearchOptions) {
		opts.Limit = limit
	}
}

// WithOffset returns a HandlerSearchOption for setting the offset for the results.
func WithOffset(offset int) HandlerSearchOption {
	return func(opts *HandlerSearchOptions) {
		opts.Offset = offset
	}
}

// WithSortBy returns a HandlerSearchOption for setting the sort order of the results.
func WithSortBy(sortBy []string) HandlerSearchOption {
	return func(opts *HandlerSearchOptions) {
		opts.SortBy = sortBy
	}
}

// WithTimeout returns a HandlerSearchOption for setting the timeout for the search.
func WithTimeout(timeout int) HandlerSearchOption {
	return func(opts *HandlerSearchOptions) {
		opts.Timeout = timeout
	}
}

// WithScoreThreshold returns a HandlerSearchOption for setting the minimum score for a result to be included.
func WithScoreThreshold(scoreThreshold float32) HandlerSearchOption {
	return func(opts *HandlerSearchOptions) {
		opts.ScoreThreshold = scoreThreshold
	}
}

// WithSelectedFields returns a HandlerSearchOption for setting the fields to return in the results.
func WithSelectedFields(selectedFields []string) HandlerSearchOption {
	return func(opts *HandlerSearchOptions) {
		opts.SelectedFields = selectedFields
	}
}

// WithRedisFilters returns a HandlerSearchOption for setting the Redis filters.
func WithRedisFilters(redisFilters string) HandlerSearchOption {
	return func(opts *HandlerSearchOptions) {
		opts.RedisFilters = redisFilters
	}
}

// WithRedisVectorFilterKey returns a HandlerSearchOption for setting the Redis vector filter key.
func WithRedisVectorFilterKey(redisVectorFilterKey string) HandlerSearchOption {
	return func(opts *HandlerSearchOptions) {
		opts.RedisVectorFilterKey = redisVectorFilterKey
	}
}

// WithRedisVectorFilterValues returns a HandlerSearchOption for setting the Redis vector filter values.
func WithRedisVectorFilterValues(redisVectorFilterValues []float32) HandlerSearchOption {
	return func(opts *HandlerSearchOptions) {
		opts.RedisVectorFilterValues = redisVectorFilterValues
	}
}

// WithRedisNoContent returns a HandlerSearchOption for setting whether to return only the IDs of the results without the content.
func WithRedisNoContent(redisNoContent bool) HandlerSearchOption {
	return func(opts *HandlerSearchOptions) {
		opts.RedisNoContent = redisNoContent
	}
}

// WithRedisVerbatim returns a HandlerSearchOption for setting whether to return the verbatim content of the results.
func WithRedisVerbatim(redisVerbatim bool) HandlerSearchOption {
	return func(opts *HandlerSearchOptions) {
		opts.RedisVerbatim = redisVerbatim
	}
}

// WithRedisWithScores returns a HandlerSearchOption for setting whether to return the score of the results.
func WithRedisWithScores(redisWithScores bool) HandlerSearchOption {
	return func(opts *HandlerSearchOptions) {
		opts.RedisWithScores = redisWithScores
	}
}

// WithRedisWithSortKeys returns a HandlerSearchOption for setting whether to return the sort keys of the results.
func WithRedisWithSortKeys(redisWithSortKeys bool) HandlerSearchOption {
	return func(opts *HandlerSearchOptions) {
		opts.RedisWithSortKeys = redisWithSortKeys
	}
}

// WithRedisInKeys returns a HandlerSearchOption for setting the Redis in keys.
func WithRedisInKeys(redisInKeys []string) HandlerSearchOption {
	return func(opts *HandlerSearchOptions) {
		opts.RedisInKeys = redisInKeys
	}
}

// WithRedisInFields returns a HandlerSearchOption for setting the Redis in fields.
func WithRedisInFields(redisInFields []string) HandlerSearchOption {
	return func(opts *HandlerSearchOptions) {
		opts.RedisInFields = redisInFields
	}
}
