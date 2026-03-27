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

	"github.com/megaease/easegress/v2/pkg/object/aigatewaycontroller/middlewares/vectordb/vecdbtypes"
)

const defaultVectorName = "embedding"

type (
	QdrantVectorDBSpec struct {
		Host   string `json:"host" jsonschema:"required"`
		Port   int    `json:"port" jsonschema:"required"`
		APIKey string `json:"apiKey,omitempty"`
		UseTLS bool   `json:"useTLS,omitempty"`
	}

	QdrantVectorDB struct {
		CommonSpec *vecdbtypes.CommonSpec
		Spec       *QdrantVectorDBSpec
	}

	QdrantVectorHandler struct {
		client         *QdrantClient
		collectionName string
		vectorName     string
	}
)

func New(common *vecdbtypes.CommonSpec, spec *QdrantVectorDBSpec) *QdrantVectorDB {
	return &QdrantVectorDB{
		CommonSpec: common,
		Spec:       spec,
	}
}

func (q *QdrantVectorDB) CreateSchema(ctx context.Context, options ...vecdbtypes.Option) (vecdbtypes.VectorHandler, error) {
	client, err := NewQdrantClient(q.Spec)
	if err != nil {
		return nil, NewErrCreateQdrantClient("failed to create Qdrant client", err)
	}

	opts := &vecdbtypes.Options{}
	for _, opt := range options {
		opt(opts)
	}

	collectionName := opts.DBName
	if collectionName == "" {
		collectionName = q.CommonSpec.CollectionName
	}

	schema, ok := opts.Schema.(*CollectionSchema)
	if !ok {
		return nil, NewErrUnexpectedCollectionSchema("unexpected schema type, expected CollectionSchema", fmt.Errorf("got %T", opts.Schema))
	}

	vectorName := schema.VectorName
	if vectorName == "" {
		vectorName = defaultVectorName
	}

	if err := client.CreateCollectionIfNotExists(ctx, collectionName, schema); err != nil {
		return nil, NewErrCreateQdrantCollection("failed to create Qdrant collection", err)
	}

	return &QdrantVectorHandler{
		client:         client,
		collectionName: collectionName,
		vectorName:     vectorName,
	}, nil
}

var _ vecdbtypes.VectorHandler = (*QdrantVectorHandler)(nil)

func (q *QdrantVectorHandler) InsertDocuments(ctx context.Context, docs []map[string]any, options ...vecdbtypes.HandlerInsertOption) ([]string, error) {
	ids, err := q.client.UpsertPoints(ctx, q.collectionName, q.vectorName, docs)
	if err != nil {
		return nil, NewErrInsertDocuments("failed to insert documents", err)
	}
	return ids, nil
}

func (q *QdrantVectorHandler) SimilaritySearch(ctx context.Context, options ...vecdbtypes.HandlerSearchOption) ([]map[string]any, error) {
	opts := &vecdbtypes.HandlerSearchOptions{}
	for _, opt := range options {
		opt(opts)
	}

	queryOpts := toQdrantQueryOptions(*opts)
	query := NewQdrantVectorQuery(q.collectionName, opts.QdrantVectorFilterValues, queryOpts...)

	docs, err := q.client.QuerySimilar(ctx, query)
	if err != nil {
		return nil, NewErrSimilaritySearch("failed to perform similarity search", err)
	}

	if len(docs) == 0 {
		return nil, vecdbtypes.ErrSimilaritySearchNotFound
	}

	return docs, nil
}

func toQdrantQueryOptions(options vecdbtypes.HandlerSearchOptions) []Option {
	opts := []Option{}

	if options.Limit > 0 {
		opts = append(opts, WithLimit(uint64(options.Limit)))
	}

	if options.Offset > 0 {
		opts = append(opts, WithOffset(uint64(options.Offset)))
	}

	if options.ScoreThreshold > 0 {
		opts = append(opts, WithScoreThreshold(options.ScoreThreshold))
	}

	return opts
}

func ValidateSpec(spec *QdrantVectorDBSpec) error {
	if spec == nil {
		return fmt.Errorf("qdrant vector spec is nil")
	}
	if spec.Host == "" {
		return fmt.Errorf("qdrant vector host is empty")
	}
	if spec.Port <= 0 {
		return fmt.Errorf("qdrant vector port must be a positive integer")
	}
	return nil
}
