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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"reflect"
	"strconv"
	"sync"

	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/object/aigatewaycontroller/aicontext"
	"github.com/megaease/easegress/v2/pkg/object/aigatewaycontroller/middlewares/embeddings"
	"github.com/megaease/easegress/v2/pkg/object/aigatewaycontroller/middlewares/vectordb"
	"github.com/megaease/easegress/v2/pkg/object/aigatewaycontroller/middlewares/vectordb/pgvector"
	"github.com/megaease/easegress/v2/pkg/object/aigatewaycontroller/middlewares/vectordb/redisvector"
	"github.com/megaease/easegress/v2/pkg/object/aigatewaycontroller/middlewares/vectordb/vecdbtypes"
)

const semanticCacheDefaultContentTemplate = `{{ $last := "" }}{{ range .messages}}{{ $last = .content }}{{ end }}{{ $last }}`

type (
	SemanticCacheSpec struct {
		Embeddings      *embeddings.EmbeddingSpec `json:"embeddings" jsonschema:"required"`
		VectorDB        *vectordb.Spec            `json:"vectorDB" jsonschema:"required"`
		ReadOnly        bool                      `json:"readOnly" jsonschema:"default=false"`
		ContentTemplate string                    `json:"contentTemplate,omitempty"`
	}

	semanticCacheMiddleware struct {
		spec              *MiddlewareSpec
		embeddingsHandler embeddings.EmbeddingHandler
		vectorHandler     *semanticCacheVectorHandler
		template          *template.Template
	}
)

func init() {
	middlewareTypeRegistry[semanticCacheMiddlewareKind] = reflect.TypeOf(semanticCacheMiddleware{})
}

var _ Middleware = (*semanticCacheMiddleware)(nil)

func (m *semanticCacheMiddleware) init(spec *MiddlewareSpec) {
	m.spec = spec
	m.embeddingsHandler = embeddings.New(spec.SemanticCache.Embeddings)
	m.vectorHandler = &semanticCacheVectorHandler{
		spec:     spec,
		dbSpec:   spec.SemanticCache.VectorDB,
		vectorDB: vectordb.New(spec.SemanticCache.VectorDB),
		handlers: make(map[string]vectordb.VectorHandler),
	}
	templateText := spec.SemanticCache.ContentTemplate
	if templateText == "" {
		templateText = semanticCacheDefaultContentTemplate
	}
	m.template = template.Must(template.New("").Parse(templateText))
}

func (m *semanticCacheMiddleware) validate(spec *MiddlewareSpec) error {
	if spec.SemanticCache == nil {
		return fmt.Errorf("semanticCache middleware %s must have a semanticCache spec", spec.Name)
	}
	if spec.SemanticCache.Embeddings == nil {
		return fmt.Errorf("semanticCache middleware %s must have an embeddings spec", spec.Name)
	}
	if spec.SemanticCache.VectorDB == nil {
		return fmt.Errorf("semanticCache middleware %s must have a vectorDB spec", spec.Name)
	}
	if err := embeddings.ValidateSpec(spec.SemanticCache.Embeddings); err != nil {
		return fmt.Errorf("semanticCache middleware %s has invalid embeddings spec: %w", spec.Name, err)
	}
	if err := vectordb.ValidateSpec(spec.SemanticCache.VectorDB); err != nil {
		return fmt.Errorf("semanticCache middleware %s has invalid vectorDB spec: %w", spec.Name, err)
	}
	return nil
}

func (m *semanticCacheMiddleware) Name() string {
	return m.spec.Name
}

func (m *semanticCacheMiddleware) Kind() string {
	return semanticCacheMiddlewareKind
}

func (m *semanticCacheMiddleware) Spec() *MiddlewareSpec {
	return m.spec
}

func (m *semanticCacheMiddleware) getContext(ctx *aicontext.Context) (string, error) {
	var result bytes.Buffer
	if err := m.template.Execute(&result, ctx.OpenAIReq); err != nil {
		return "", fmt.Errorf("failed to execute template for semantic cache: %w", err)
	}
	if result.Len() == 0 {
		return "", fmt.Errorf("the content template for semantic cache is empty")
	}
	return result.String(), nil
}

func (m *semanticCacheMiddleware) addInsertCacheCallback(ctx *aicontext.Context, embedding []float32) {
	if m.spec.SemanticCache.ReadOnly {
		return
	}

	ctx.AddCallBack(func(fc *aicontext.FinishContext) {
		if fc.StatusCode != 200 {
			return
		}
		handler, err := m.vectorHandler.GetHandler(ctx, embedding)
		if err != nil {
			logger.Errorf("failed to get vector handler for semantic cache: %v", err)
			return
		}

		header, err := json.Marshal(fc.Header)
		if err != nil {
			logger.Errorf("failed to marshal response header: %v", err)
			return
		}
		cache := map[string]any{
			"embedding": embedding,
			"data":      string(fc.RespBody),
			"header":    string(header),
			"status":    fc.StatusCode,
		}
		handler.InsertDocuments(ctx.Req.Std().Context(), []map[string]any{cache})
	})
}

func (m *semanticCacheMiddleware) writeRespWithCache(ctx *aicontext.Context, cache map[string]any) {
	defer func() {
		if r := recover(); r != nil {
			if err, ok := r.(error); ok {
				logger.Errorf("panic in writeRespWithCache: %v", err)
			} else {
				logger.Errorf("panic in writeRespWithCache: %v", r)
			}
		}
	}()

	data := cache["data"].(string)
	headerStr := cache["header"].(string)
	status := cache["status"].(int)

	h := http.Header{}
	if err := json.Unmarshal([]byte(headerStr), &h); err != nil {
		logger.Errorf("failed to unmarshal response header: %v", err)
		return
	}

	resp := &aicontext.Response{
		StatusCode: status,
		Header:     h,
	}
	if ctx.ReqInfo.Stream {
		resp.BodyReader = bytes.NewReader([]byte(data))
	} else {
		resp.BodyBytes = []byte(data)
		resp.ContentLength = int64(len(resp.BodyBytes))
	}
	ctx.SetResponse(resp)
	ctx.Stop("")
}

func (m *semanticCacheMiddleware) Handle(ctx *aicontext.Context) {
	if ctx.RespType != aicontext.ResponseTypeChatCompletions && ctx.RespType != aicontext.ResponseTypeCompletions {
		return
	}

	context, err := m.getContext(ctx)
	if err != nil {
		logger.Errorf("failed to get context for semantic cache: %v", err)
		return
	}
	embedding, err := m.embeddingsHandler.EmbedQuery(context)
	if err != nil {
		logger.Errorf("failed to embed context for semantic cache: %v", err)
		return
	}
	handler, err := m.vectorHandler.GetHandler(ctx, embedding)
	if err != nil {
		logger.Errorf("failed to get vector handler for semantic cache: %v", err)
		return
	}
	cache, err := handler.SimilaritySearch(
		ctx.Req.Std().Context(),
		m.getSearchOptions(ctx, embedding)...,
	)
	if err != nil {
		if err == vectordb.ErrSimilaritySearchNotFound {
			m.addInsertCacheCallback(ctx, embedding)
			return
		}
		logger.Errorf("failed to search similarity in vector database: %v", err)
		return
	}
	if len(cache) == 0 {
		m.addInsertCacheCallback(ctx, embedding)
		return
	}
	m.writeRespWithCache(ctx, cache[0])
}

func (m *semanticCacheMiddleware) getSearchOptions(ctx *aicontext.Context, embedding []float32) []vecdbtypes.HandlerSearchOption {
	switch m.spec.SemanticCache.VectorDB.Type {
	case vectordb.TypePostgres:
		return []vecdbtypes.HandlerSearchOption{
			vecdbtypes.WithPostgresVectorFilterKey("embedding"),
			vecdbtypes.WithPostgresVectorFilterValues(embedding),
			vecdbtypes.WithScoreThreshold(float32(m.spec.SemanticCache.VectorDB.Threshold)),
		}
	case vectordb.TypeRedis:
		return []vecdbtypes.HandlerSearchOption{
			vecdbtypes.WithRedisVectorFilterKey("embedding"),
			vecdbtypes.WithRedisVectorFilterValues(embedding),
			vecdbtypes.WithScoreThreshold(float32(m.spec.SemanticCache.VectorDB.Threshold)),
		}
	default:
		panic(fmt.Sprintf("unsupported vector db type: %s", m.spec.SemanticCache.VectorDB.Type))
	}
}

type (
	semanticCacheVectorHandler struct {
		spec        *MiddlewareSpec
		dbSpec      *vectordb.Spec
		vectorDB    vectordb.VectorDB
		handlerLock sync.RWMutex
		handlers    map[string]vectordb.VectorHandler
	}
)

func (h *semanticCacheVectorHandler) getHandlerKey(ctx *aicontext.Context) string {
	return string(ctx.RespType) + strconv.FormatBool(ctx.ReqInfo.Stream)
}

func (h *semanticCacheVectorHandler) GetHandler(ctx *aicontext.Context, embedding []float32) (vectordb.VectorHandler, error) {
	key := h.getHandlerKey(ctx)

	h.handlerLock.RLock()
	if handler, exists := h.handlers[key]; exists {
		h.handlerLock.RUnlock()
		return handler, nil
	}
	h.handlerLock.RUnlock()

	h.handlerLock.Lock()
	defer h.handlerLock.Unlock()
	// re-check to avoid in race condition re-create the handler
	if handler, exists := h.handlers[key]; exists {
		return handler, nil
	}

	handler, err := h.vectorDB.CreateSchema(context.Background(), h.createOptions(ctx, embedding))
	if err != nil {
		return nil, fmt.Errorf("failed to create index, %v", err)
	}
	h.handlers[key] = handler
	return handler, nil
}

func (h *semanticCacheVectorHandler) createOptions(ctx *aicontext.Context, embedding []float32) vecdbtypes.Option {
	switch h.dbSpec.Type {
	case vectordb.TypePostgres:
		return h.createPostgresOptions(ctx, embedding)
	case vectordb.TypeRedis:
		return h.createRedisOptions(ctx, embedding)
	default:
		// should not reach here, since we validate the spec before creating the handler.
		panic(fmt.Sprintf("unsupported vector db type: %s", h.dbSpec.Type))
	}
}

func (h *semanticCacheVectorHandler) createRedisOptions(ctx *aicontext.Context, embedding []float32) vecdbtypes.Option {
	return func(o *vecdbtypes.Options) {
		o.DBName = h.getRedisDBName(ctx)
		o.Schema = h.createRedisSchema(len(embedding))
	}
}

func (h *semanticCacheVectorHandler) getRedisDBName(ctx *aicontext.Context) string {
	spec := h.spec.SemanticCache.VectorDB
	dbName := spec.CollectionName
	switch ctx.RespType {
	case aicontext.ResponseTypeChatCompletions:
		dbName += "_chat"
	case aicontext.ResponseTypeCompletions:
		dbName += "_completion"
	default:
		// should not reach here, check code in semanticCacheMiddleware
		panic(fmt.Sprintf("unsupported response type: %s", ctx.RespType))
	}
	if ctx.ReqInfo.Stream {
		dbName += "_stream"
	} else {
		dbName += "_non_stream"
	}
	return dbName
}

func (h *semanticCacheVectorHandler) createRedisSchema(dim int) vecdbtypes.Schema {
	return &redisvector.IndexSchema{
		Vectors: []redisvector.Vector{
			{
				Name: "embedding",
				Dim:  dim,
			},
		},
		Texts: []redisvector.Text{
			{
				Name: "data",
			},
			{
				Name: "header",
			},
		},
		Numerics: []redisvector.Numeric{
			{
				Name: "status",
			},
		},
	}
}

func (h *semanticCacheVectorHandler) createPostgresOptions(ctx *aicontext.Context, embedding []float32) vecdbtypes.Option {
	return func(o *vecdbtypes.Options) {
		o.DBName = h.dbSpec.CollectionName
		o.Schema = h.createPostgresSchema(ctx, embedding)
	}
}

func (h *semanticCacheVectorHandler) getPostgresTableName(ctx *aicontext.Context) string {
	tableName := "semantic_cache"
	switch ctx.RespType {
	case aicontext.ResponseTypeChatCompletions:
		tableName += "_chat"
	case aicontext.ResponseTypeCompletions:
		tableName += "_completion"
	default:
		// should not reach here, check code in semanticCacheMiddleware
		panic(fmt.Sprintf("unsupported response type: %s", ctx.RespType))
	}
	if ctx.ReqInfo.Stream {
		tableName += "_stream"
	} else {
		tableName += "_non_stream"
	}
	return tableName
}

func (h *semanticCacheVectorHandler) createPostgresSchema(ctx *aicontext.Context, embedding []float32) vecdbtypes.Schema {
	return &pgvector.TableSchema{
		TableName: h.getPostgresTableName(ctx),
		Columns: []pgvector.Column{
			{Name: "embedding", DataType: fmt.Sprintf("vector(%d)", len(embedding))},
			{Name: "data", DataType: "text"},
			{Name: "header", DataType: "text"},
			{Name: "status", DataType: "int"},
		},
	}
}
