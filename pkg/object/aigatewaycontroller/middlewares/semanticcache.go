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
	"fmt"
	"reflect"

	"github.com/megaease/easegress/v2/pkg/object/aigatewaycontroller/aicontext"
	"github.com/megaease/easegress/v2/pkg/object/aigatewaycontroller/middlewares/embeddings"
	"github.com/megaease/easegress/v2/pkg/object/aigatewaycontroller/middlewares/vectordb"
)

type (
	SemanticCacheSpec struct {
		Embeddings *embeddings.EmbeddingSpec `json:"embeddings" jsonschema:"required"`
		VectorDB   *vectordb.Spec            `json:"vectorDB" jsonschema:"required"`
	}

	semanticCacheMiddleware struct {
		spec              *MiddlewareSpec
		embeddingsHandler embeddings.EmbeddingHandler
		vectorDB          vectordb.VectorDB
	}
)

func init() {
	middlewareTypeRegistry[semanticCacheMiddlewareKind] = reflect.TypeOf(semanticCacheMiddleware{})
}

var _ Middleware = (*semanticCacheMiddleware)(nil)

func (m *semanticCacheMiddleware) init(spec *MiddlewareSpec) {
	m.spec = spec
	m.embeddingsHandler = embeddings.New(spec.SemanticCache.Embeddings)
	m.vectorDB = vectordb.New(spec.SemanticCache.VectorDB)
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

func (m *semanticCacheMiddleware) Handle(ctx *aicontext.Context) {
	// TODO
}
