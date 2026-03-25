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
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/qdrant/go-client/qdrant"
)

type QdrantClient struct {
	client *qdrant.Client
}

func NewQdrantClient(spec *QdrantVectorDBSpec) (*QdrantClient, error) {
	client, err := qdrant.NewClient(&qdrant.Config{
		Host:   spec.Host,
		Port:   spec.Port,
		APIKey: spec.APIKey,
		UseTLS: spec.UseTLS,
	})
	if err != nil {
		return nil, err
	}
	return &QdrantClient{client: client}, nil
}

func (c *QdrantClient) CollectionExists(ctx context.Context, name string) (bool, error) {
	return c.client.CollectionExists(ctx, name)
}

func (c *QdrantClient) CreateCollectionIfNotExists(ctx context.Context, name string, schema *CollectionSchema) error {
	exists, err := c.CollectionExists(ctx, name)
	if err != nil {
		return fmt.Errorf("failed to check if collection exists: %w", err)
	}
	if exists {
		return nil
	}

	distance := toQdrantDistance(schema.Distance)
	err = c.client.CreateCollection(ctx, &qdrant.CreateCollection{
		CollectionName: name,
		VectorsConfig:  qdrant.NewVectorsConfig(&qdrant.VectorParams{Size: schema.VectorSize, Distance: distance}),
	})
	if err != nil {
		return fmt.Errorf("failed to create collection %s: %w", name, err)
	}
	return nil
}

func (c *QdrantClient) UpsertPoints(ctx context.Context, collectionName string, vectorName string, docs []map[string]any) ([]string, error) {
	if len(docs) == 0 {
		return []string{}, nil
	}

	points := make([]*qdrant.PointStruct, 0, len(docs))
	ids := make([]string, 0, len(docs))

	for _, doc := range docs {
		rawVec, ok := doc[vectorName]
		if !ok {
			return nil, fmt.Errorf("document missing vector field %q", vectorName)
		}
		vector, ok := rawVec.([]float32)
		if !ok {
			return nil, fmt.Errorf("vector field %q is not []float32", vectorName)
		}

		payload := make(map[string]any, len(doc)-1)
		for k, v := range doc {
			if k != vectorName {
				payload[k] = v
			}
		}

		pointID := uuid.New().String()
		ids = append(ids, pointID)

		valueMap, err := qdrant.TryValueMap(payload)
		if err != nil {
			return nil, fmt.Errorf("failed to convert payload for document: %w", err)
		}
		points = append(points, &qdrant.PointStruct{
			Id:      qdrant.NewID(pointID),
			Vectors: qdrant.NewVectors(vector...),
			Payload: valueMap,
		})
	}

	_, err := c.client.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: collectionName,
		Points:         points,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to upsert points into collection %s: %w", collectionName, err)
	}

	return ids, nil
}

func (c *QdrantClient) QuerySimilar(ctx context.Context, query *QdrantVectorQuery) ([]map[string]any, error) {
	scoredPoints, err := c.client.Query(ctx, &qdrant.QueryPoints{
		CollectionName: query.collectionName,
		Query:          qdrant.NewQuery(query.vectorValues...),
		WithPayload:    qdrant.NewWithPayload(true),
		Limit:          qdrant.PtrOf(query.limit),
		Offset:         qdrant.PtrOf(query.offset),
		ScoreThreshold: qdrant.PtrOf(query.scoreThreshold),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query collection %s: %w", query.collectionName, err)
	}

	results := make([]map[string]any, 0, len(scoredPoints))
	for _, sp := range scoredPoints {
		doc := make(map[string]any, len(sp.Payload)+1)
		for k, v := range sp.Payload {
			doc[k] = qdrantValueToGo(v)
		}
		doc["score"] = sp.Score
		results = append(results, doc)
	}

	return results, nil
}

func (c *QdrantClient) Close() error {
	return c.client.Close()
}

func qdrantValueToGo(v *qdrant.Value) any {
	if v == nil {
		return nil
	}
	switch kind := v.Kind.(type) {
	case *qdrant.Value_StringValue:
		return kind.StringValue
	case *qdrant.Value_IntegerValue:
		return int(kind.IntegerValue)
	case *qdrant.Value_DoubleValue:
		return kind.DoubleValue
	case *qdrant.Value_BoolValue:
		return kind.BoolValue
	case *qdrant.Value_NullValue:
		return nil
	default:
		return nil
	}
}
