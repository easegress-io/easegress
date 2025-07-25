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

const (
	// IndexTypeHash represents a hash index type.
	IndexTypeHash IndexType = "HASH"
	// IndexTypeJSON represents a JSON index type.
	IndexTypeJSON IndexType = "JSON"
	// IndexTypeHNSW represents a HNSW index type.
	IndexTypeHNSW IndexType = "HNSW"
	// IndexTypeIVFFlat represents a IVFFlat index type.
	IndexTypeIVFFlat IndexType = "IVFFlat"
)

var (
	validHNSWDistanceMetrics = []string{
		"vector_l2_ops",     // L2 distance
		"vector_ip_ops",     // Inner product
		"vector_cosine_ops", // Cosine distance
		"vector_l1_ops",     // L1 distance
		"bit_hamming_ops",   // Hamming distance
		"bit_jaccard_ops",   // Jaccard distance
	}

	validIVFFlatDistanceMetrics = []string{
		"vector_l2_ops",     // L2 distance
		"vector_ip_ops",     // Inner product
		"vector_cosine_ops", // Cosine distance
		"bit_hamming_ops",   // Hamming distance
	}
)

type (
	TableSchema struct {
		TableName string   `json:"tableName" jsonschema:"required"`
		Columns   []Column `json:"columns" jsonschema:"required"`
		Indexes   []Index  `json:"indexes,omitempty" jsonschema:"optional"`
	}

	Column struct {
		Name         string `json:"name" jsonschema:"required"`
		DataType     string `json:"dataType" jsonschema:"required"`
		IsPrimary    bool   `json:"isPrimary" jsonschema:"required"`
		IsUnique     bool   `json:"isUnique" jsonschema:"required"`
		IsNullable   bool   `json:"isNullable" jsonschema:"required"`
		DefaultValue string `json:"defaultValue,omitempty" jsonschema:"optional"`
	}

	IndexType string

	Index struct {
		Name    string              `json:"name" jsonschema:"required"`
		Type    IndexType           `json:"type" jsonschema:"required"`
		Column  string              `json:"column" jsonschema:"required"`
		HNSW    *HNSWIndexOption    `json:"hnsw,omitempty" jsonschema:"optional"`
		IVFFlat *IVFFlatIndexOption `json:"ivfflat,omitempty" jsonschema:"optional"`
	}

	HNSWIndexOption struct {
		DistanceMetric string `json:"distanceMetric" jsonschema:"required"`
		M              int    `json:"m" jsonschema:"required"`
		EfConstruction int    `json:"efConstruction" jsonschema:"required"`
	}

	IVFFlatIndexOption struct {
		DistanceMetric string `json:"distanceMetric" jsonschema:"required"`
		Nlist          int    `json:"nlist" jsonschema:"required"`
	}
)

func (t *TableSchema) SchemaType() string {
	return "pgvector"
}
