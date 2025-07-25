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

package redisvector

type ErrParsingRedisURL struct {
	Message string
	Err     error
}

// NewErrParsingRedisURL creates a new ErrParsingRedisURL with the given message and error.
func NewErrParsingRedisURL(message string, err error) *ErrParsingRedisURL {
	return &ErrParsingRedisURL{
		Message: message,
		Err:     err,
	}
}

func (e *ErrParsingRedisURL) Error() string {
	return e.Message + ": " + e.Err.Error()
}

type ErrCreateRedisClient struct {
	Message string
	Err     error
}

// NewErrCreateRedisClient creates a new ErrCreateRedisClient with the given message and error.
func NewErrCreateRedisClient(message string, err error) *ErrCreateRedisClient {
	return &ErrCreateRedisClient{
		Message: message,
		Err:     err,
	}
}

func (e *ErrCreateRedisClient) Error() string {
	return e.Message + ": " + e.Err.Error()
}

type ErrUnexpectedIndexSchema struct {
	Message string
	Err     error
}

// NewErrUnexpectedIndexSchema creates a new ErrUnexpectedIndexSchema with the given message and error.
func NewErrUnexpectedIndexSchema(message string, err error) *ErrUnexpectedIndexSchema {
	return &ErrUnexpectedIndexSchema{
		Message: message,
		Err:     err,
	}
}

func (e *ErrUnexpectedIndexSchema) Error() string {
	return e.Message + ": " + e.Err.Error()
}

type ErrCreateRedisIndex struct {
	Message string
	Err     error
}

// NewErrCreateRedisIndex creates a new ErrCreateRedisIndex with the given message and error.
func NewErrCreateRedisIndex(message string, err error) *ErrCreateRedisIndex {
	return &ErrCreateRedisIndex{
		Message: message,
		Err:     err,
	}
}

func (e *ErrCreateRedisIndex) Error() string {
	return e.Message + ": " + e.Err.Error()
}

type ErrInsertDocument struct {
	Message string
	Err     error
}

// NewErrInsertDocument creates a new ErrInsertDocument with the given message and error.
func NewErrInsertDocument(message string, err error) *ErrInsertDocument {
	return &ErrInsertDocument{
		Message: message,
		Err:     err,
	}
}

func (e *ErrInsertDocument) Error() string {
	return e.Message + ": " + e.Err.Error()
}

type InvalidScoreThreshold struct{}

// NewInvalidScoreThreshold creates a new InvalidScoreThreshold error.
func NewInvalidScoreThreshold() *InvalidScoreThreshold {
	return &InvalidScoreThreshold{}
}

func (e *InvalidScoreThreshold) Error() string {
	return "invalid score threshold: must be between 0 and 1"
}
