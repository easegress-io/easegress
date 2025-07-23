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

package openai

import (
	"crypto/sha512"
	"encoding/binary"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/object/aigatewaycontroller/middlewares/embeddings/embedtypes"
	"github.com/megaease/easegress/v2/pkg/object/aigatewaycontroller/protocol"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	logger.InitNop()
	code := m.Run()
	os.Exit(code)
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

func TestOpenAIEmbedding(t *testing.T) {
	assert := assert.New(t)

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusInternalServerError)
			return
		}
		embedReq := protocol.EmbedRequest{}
		if err := json.Unmarshal(body, &embedReq); err != nil {
			http.Error(w, "Failed to parse request body", http.StatusBadRequest)
			return
		}
		input := embedReq.Input.(string)
		embedVec := embeddingString(input)
		resp := protocol.EmbeddingResponse{
			Object: "list",
			Data: []protocol.Embedding{{
				Object:    "embedding",
				Embedding: embedVec,
				Index:     0,
			}},
			Model: embedReq.Model,
			Usage: protocol.EmbeddingUsage{PromptTokens: len(input), TotalTokens: len(input)},
		}

		respBody, err := json.Marshal(resp)
		if err != nil {
			http.Error(w, "Failed to marshal response", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(respBody)
	}))
	defer mockServer.Close()

	spec := &embedtypes.EmbeddingSpec{
		ProviderType: "openai",
		BaseURL:      mockServer.URL,
		Model:        "test-model",
		APIKey:       "mock-api",
	}
	handler := New(spec)
	embed, err := handler.EmbedQuery("hello world")
	assert.Nil(err)
	embed2, err := handler.EmbedDocuments("hello world2")
	assert.Nil(err)

	assert.NotEqual(embed, embed2)
	assert.Equal(embeddingString("hello world"), embed)
	assert.Equal(embeddingString("hello world2"), embed2)
}
