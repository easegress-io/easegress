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

import (
	"github.com/qdrant/go-client/qdrant"

	"github.com/megaease/easegress/v2/pkg/object/aigatewaycontroller/middlewares/vectordb/vecdbtypes"
)

type Distance string

const (
	DistanceCosine    Distance = "Cosine"
	DistanceDot       Distance = "Dot"
	DistanceEuclid    Distance = "Euclid"
	DistanceManhattan Distance = "Manhattan"
)

type CollectionSchema struct {
	VectorSize uint64   `json:"vectorSize" jsonschema:"required"`
	Distance   Distance `json:"distance,omitempty"`
	VectorName string   `json:"vectorName,omitempty"`
}

func (c *CollectionSchema) SchemaType() string {
	return "qdrant"
}

var _ vecdbtypes.Schema = (*CollectionSchema)(nil)

func toQdrantDistance(d Distance) qdrant.Distance {
	switch d {
	case DistanceDot:
		return qdrant.Distance_Dot
	case DistanceEuclid:
		return qdrant.Distance_Euclid
	case DistanceManhattan:
		return qdrant.Distance_Manhattan
	case DistanceCosine:
		fallthrough
	default:
		return qdrant.Distance_Cosine
	}
}
