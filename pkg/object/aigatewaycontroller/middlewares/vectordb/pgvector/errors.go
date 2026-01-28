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

type ErrCreatePostgresClient struct {
	Message string
	Err     error
}

// NewErrCreatePostgresClient creates a new ErrCreatePostgresClient with the given message and error.
func NewErrCreatePostgresClient(message string, err error) *ErrCreatePostgresClient {
	return &ErrCreatePostgresClient{
		Message: message,
		Err:     err,
	}
}

func (e *ErrCreatePostgresClient) Error() string {
	return e.Message + ": " + e.Err.Error()
}

type ErrEnableVectorExtension struct {
	Message string
	Err     error
}

// NewErrEnableVectorExtension creates a new ErrEnableVectorExtension with the given message and error.
func NewErrEnableVectorExtension(message string, err error) *ErrEnableVectorExtension {
	return &ErrEnableVectorExtension{
		Message: message,
		Err:     err,
	}
}

func (e *ErrEnableVectorExtension) Error() string {
	return e.Message + ": " + e.Err.Error()
}

type ErrBeginTransaction struct {
	Message string
	Err     error
}

// NewErrBeginTransaction creates a new ErrBeginTransaction with the given message and error.
func NewErrBeginTransaction(message string, err error) *ErrBeginTransaction {
	return &ErrBeginTransaction{
		Message: message,
		Err:     err,
	}
}

func (e *ErrBeginTransaction) Error() string {
	return e.Message + ": " + e.Err.Error()
}

type ErrUnexpectedSchemaType struct {
	Message string
	Err     error
}

// NewErrUnexpectedSchemaType creates a new ErrUnexpectedSchemaType with the given message and error.
func NewErrUnexpectedSchemaType(message string, err error) *ErrUnexpectedSchemaType {
	return &ErrUnexpectedSchemaType{
		Message: message,
		Err:     err,
	}
}

func (e *ErrUnexpectedSchemaType) Error() string {
	return e.Message + ": " + e.Err.Error()
}

type ErrCreatePostgresDB struct {
	Message string
	Err     error
}

// NewErrCreatePostgresDB creates a new ErrCreatePostgresDB with the given message and error.
func NewErrCreatePostgresDB(message string, err error) *ErrCreatePostgresDB {
	return &ErrCreatePostgresDB{
		Message: message,
		Err:     err,
	}
}

func (e *ErrCreatePostgresDB) Error() string {
	return e.Message + ": " + e.Err.Error()
}

type ErrInsertDocuments struct {
	Message string
	Err     error
}

// NewErrInsertDocuments creates a new ErrInsertDocuments with the given message and error.
func NewErrInsertDocuments(message string, err error) *ErrInsertDocuments {
	return &ErrInsertDocuments{
		Message: message,
		Err:     err,
	}
}

func (e *ErrInsertDocuments) Error() string {
	return e.Message + ": " + e.Err.Error()
}
