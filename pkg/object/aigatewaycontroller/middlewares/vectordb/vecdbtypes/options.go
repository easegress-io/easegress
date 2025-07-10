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

package vecdbtypes

type Option func(*Options)

// Options is the struct for options.
type Options struct {
	// DBName is the name of the vector database.
	DBName string
	// ScoreThreshold is the threshold for the similarity score.
	ScoreThreshold float32
	// Filters is the metadata filters to afpply.
	Filters interface{}
	// // EmbedHandler is the handler to use for embedding documents.
	// EmbedHandler embeddings.EmbeddingHandler
}

// // WithEmbedHandler returns an Option for setting the embedding handler.
// func WithEmbedHandler(embedHandler embeddings.EmbeddingHandler) Option {
// 	return func(o *Options) {
// 		o.EmbedHandler = embedHandler
// 	}
// }
