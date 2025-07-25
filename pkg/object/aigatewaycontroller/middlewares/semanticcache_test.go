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
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"testing"
	"time"

	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/object/aigatewaycontroller/aicontext"
	"github.com/megaease/easegress/v2/pkg/object/aigatewaycontroller/middlewares/embeddings/embedtypes"
	"github.com/megaease/easegress/v2/pkg/object/aigatewaycontroller/middlewares/vectordb"
	"github.com/megaease/easegress/v2/pkg/object/aigatewaycontroller/middlewares/vectordb/redisvector"
	"github.com/megaease/easegress/v2/pkg/object/aigatewaycontroller/middlewares/vectordb/vecdbtypes"
	"github.com/stretchr/testify/assert"
)

func TestSemanticCache(t *testing.T) {
	assert := assert.New(t)

	spec := &MiddlewareSpec{
		Name: "test-semantic-cache",
		Kind: semanticCacheMiddlewareKind,
		SemanticCache: &SemanticCacheSpec{
			Embeddings: &embedtypes.EmbeddingSpec{
				ProviderType: "openai",
				BaseURL:      "http://localhost:8080",
				Model:        "text-embedding-3-small",
				APIKey:       "test-api-key",
			},
			VectorDB: &vectordb.Spec{
				CommonSpec: vecdbtypes.CommonSpec{
					Type:           "redis",
					Threshold:      0.99,
					CollectionName: "redis-test",
				},
				Redis: &redisvector.RedisVectorDBSpec{
					URL: "redis://localhost:6379",
				},
			},
			ReadOnly:        false,
			ContentTemplate: semanticCacheDefaultContentTemplate,
		},
	}

	cache := &semanticCacheMiddleware{}
	cache.spec = spec
	cache.embeddingsHandler = &mockEmbeddingHandler{}
	cache.vectorHandler = &semanticCacheVectorHandler{
		spec:     spec,
		dbSpec:   spec.SemanticCache.VectorDB,
		vectorDB: &mockVectorDB{},
		handlers: make(map[string]vectordb.VectorHandler),
	}
	cache.template = template.Must(template.New("").Parse(spec.SemanticCache.ContentTemplate))

	data := map[string]any{
		"model": "gpt-4.1",
		"messages": []map[string]any{
			{
				"role":    "developer",
				"content": "You are a helpful assistant.",
			},
			{
				"role":    "user",
				"content": "Hello!",
			},
		},
	}
	jsonData, err := json.Marshal(data)
	assert.Nil(err)

	providerSpec := &aicontext.ProviderSpec{
		Name:         "openai",
		ProviderType: "openai",
		APIKey:       "test-api-key",
	}

	{
		// test template
		var result bytes.Buffer
		err := cache.template.Execute(&result, data)
		assert.Nil(err)
		assert.Equal("Hello!", result.String())
	}

	{
		// no cache
		ctx := context.New(nil)
		req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:8080/v1/chat/completions", bytes.NewReader(jsonData))
		assert.Nil(err)
		setRequest(t, ctx, "no.cache", req)
		aiCtx, err := aicontext.New(ctx, providerSpec)
		assert.Nil(err)
		cache.Handle(aiCtx)
		assert.False(aiCtx.IsStopped())

		// store response
		respData := getNonStreamBody("gpt-4.1")
		respJson, err := json.Marshal(respData)
		assert.Nil(err)
		callbacks := aiCtx.Callbacks()
		for _, cb := range callbacks {
			cb(&aicontext.FinishContext{
				StatusCode: http.StatusOK,
				RespBody:   respJson,
				Duration:   100,
			})
		}
	}
	{
		// cache hit
		ctx := context.New(nil)
		req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:8080/v1/chat/completions", bytes.NewReader(jsonData))
		assert.Nil(err)
		setRequest(t, ctx, "no.cache", req)
		aiCtx, err := aicontext.New(ctx, providerSpec)
		assert.Nil(err)
		cache.Handle(aiCtx)
		assert.True(aiCtx.IsStopped())
		assert.Equal(aicontext.ResultOk, aiCtx.Result())
	}
}

func getNonStreamBody(model string) any {
	return map[string]any{
		"id":      fmt.Sprintf("chatcmpl-%d", time.Now().Unix()),
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"model":   model,
		"choices": []any{
			map[string]any{
				"index": 0,
				"message": map[string]any{
					"role":    "assistant",
					"content": "Hello! How can I assist you today?",
				},
				"finish_reason": "stop",
			},
		},
		"usage": map[string]any{
			"prompt_tokens":     19,
			"completion_tokens": 10,
			"total_tokens":      29,
			"prompt_tokens_details": map[string]any{
				"cached_tokens": 0,
				"audio_tokens":  0,
			},
			"completion_tokens_details": map[string]any{
				"reasoning_tokens":           0,
				"audio_tokens":               0,
				"accepted_prediction_tokens": 0,
				"rejected_prediction_tokens": 0,
			},
		},
		"service_tier": "default",
	}
}
