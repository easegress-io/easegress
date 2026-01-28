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

package middlewares

import (
	"context"
	"crypto/sha512"
	"encoding/binary"
	"math"
	"net/http"
	"os"
	"testing"

	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/object/aigatewaycontroller/middlewares/embeddings"
	"github.com/megaease/easegress/v2/pkg/object/aigatewaycontroller/middlewares/vectordb/vecdbtypes"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot"
	"github.com/stretchr/testify/assert"

	egContext "github.com/megaease/easegress/v2/pkg/context"
)

func TestMain(m *testing.M) {
	logger.InitNop()
	code := m.Run()
	os.Exit(code)
}

type mockEmbeddingHandler struct {
}

var _ embeddings.EmbeddingHandler = &mockEmbeddingHandler{}

func (e *mockEmbeddingHandler) EmbedDocuments(text string) ([]float32, error) {
	return embeddingString(text), nil
}

func (e *mockEmbeddingHandler) EmbedQuery(text string) ([]float32, error) {
	return embeddingString(text), nil
}

func embeddingString(s string) []float32 {
	hash := sha512.Sum512([]byte(s))

	l := 16
	vec := make([]float32, l)
	for i := range l {
		bits := binary.LittleEndian.Uint32(hash[i*4 : (i+1)*4])
		f := float32(bits) / float32(math.MaxUint32)
		vec[i] = f
	}
	return vec
}

type mockVectorDB struct {
	data []map[string]any
}

func (db *mockVectorDB) CreateSchema(ctx context.Context, options ...vecdbtypes.Option) (vecdbtypes.VectorHandler, error) {
	return db, nil
}

func (db *mockVectorDB) InsertDocuments(ctx context.Context, doc []map[string]any, options ...vecdbtypes.HandlerInsertOption) ([]string, error) {
	db.data = append(db.data, doc...)
	return nil, nil
}

func (db *mockVectorDB) SimilaritySearch(ctx context.Context, options ...vecdbtypes.HandlerSearchOption) ([]map[string]any, error) {
	opts := &vecdbtypes.HandlerSearchOptions{}
	for _, opt := range options {
		opt(opts)
	}

	key := opts.RedisVectorFilterKey
	vec := opts.RedisVectorFilterValues
	for _, doc := range db.data {
		gotVec := doc[key].([]float32)
		for i := range vec {
			if gotVec[i] != vec[i] {
				continue
			}
		}
		return []map[string]any{doc}, nil
	}
	return nil, vecdbtypes.ErrSimilaritySearchNotFound
}

func setRequest(t *testing.T, ctx *egContext.Context, ns string, req *http.Request) {
	httpreq, err := httpprot.NewRequest(req)
	httpreq.FetchPayload(0)
	assert.Nil(t, err)
	ctx.SetRequest(ns, httpreq)
	ctx.UseNamespace(ns)
}
