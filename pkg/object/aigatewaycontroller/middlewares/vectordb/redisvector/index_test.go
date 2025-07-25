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

package redisvector

import (
	"strings"
	"testing"
)

func TestTagToCommand(t *testing.T) {
	tests := []struct {
		name    string
		tag     Tag
		command string
	}{
		{
			name: "simple tag",
			tag: Tag{
				Name: "movie genre",
			},
			command: "movie genre TAG SEPARATOR ,",
		},
		{
			name: "tag with all options",
			tag: Tag{
				Name:          "movie genre",
				As:            "movie_genre",
				Separator:     "|",
				CaseSensitive: true,
				Sortable:      true,
				NoIndex:       true,
				IndexMissing:  true,
				IndexEmpty:    true,
			},
			command: "movie genre AS movie_genre TAG SEPARATOR | CASESENSITIVE SORTABLE NOINDEX INDEXMISSING INDEXEMPTY",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.tag.ToCommand()
			if strings.Join(got, " ") != tt.command {
				t.Errorf("Tag.ToCommand() = %v, want %v", got, tt.command)
			}
		})
	}
}

func TestTextToCommand(t *testing.T) {
	tests := []struct {
		name    string
		text    Text
		command string
	}{
		{
			name: "simple text",
			text: Text{
				Name: "movie name",
			},
			command: "movie name TEXT",
		},
		{
			name: "text with all options",
			text: Text{
				Name:           "movie name",
				As:             "name",
				Weight:         1.5,
				WithSuffixTire: true,
				Sortable:       true,
				NoIndex:        true,
				Phonetic:       "dm:es",
				IndexMissing:   true,
				IndexEmpty:     true,
			},
			command: "movie name AS name TEXT WEIGHT 1.5 WITHSUFFIXTRIE SORTABLE NOINDEX PHONETIC dm:es INDEXMISSING INDEXEMPTY",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.text.ToCommand()
			if strings.Join(got, " ") != tt.command {
				t.Errorf("Text.ToCommand() = %v, want %v", got, tt.command)
			}
		})
	}
}

func TestNumericToCommand(t *testing.T) {
	tests := []struct {
		name    string
		numeric Numeric
		command string
	}{
		{
			name: "simple numeric",
			numeric: Numeric{
				Name: "movie rating",
			},
			command: "movie rating NUMERIC",
		},
		{
			name: "numeric with all options",
			numeric: Numeric{
				Name:         "movie rating",
				As:           "rating",
				NoIndex:      true,
				Sortable:     true,
				IndexMissing: true,
			},
			command: "movie rating AS rating NUMERIC SORTABLE INDEXMISSING NOINDEX",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.numeric.ToCommand()
			if strings.Join(got, " ") != tt.command {
				t.Errorf("Numeric.ToCommand() = %v, want %v", got, tt.command)
			}
		})
	}
}

func TestVectorToCommand(t *testing.T) {
	tests := []struct {
		name    string
		vector  Vector
		command string
	}{
		{
			name: "simple vector",
			vector: Vector{
				Name: "movie vector",
			},
			command: "movie vector VECTOR FLAT 6 TYPE FLOAT32 DIM 128 DISTANCE_METRIC COSINE",
		},
		{
			name: "vector with all options",
			vector: Vector{
				Name:           "movie vector",
				As:             "vector",
				Dim:            256,
				Algorithm:      "HNSW",
				DistanceMetric: "L2",
				VectorType:     "BFLOAT16",
				M:              16,
				EfConstruction: 200,
				EfRuntime:      2000,
				Epsilon:        0.1,
			},
			command: "movie vector AS vector VECTOR HNSW 14 TYPE BFLOAT16 DIM 256 DISTANCE_METRIC L2 M 16 EF_CONSTRUCTION 200 EF_RUNTIME 2000 EPSILON 0.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.vector.ToCommand()
			if strings.Join(got, " ") != tt.command {
				t.Errorf("Vector.ToCommand() = %v, want %v", got, tt.command)
			}
		})
	}
}

func TestIndexSchemaToCommand(t *testing.T) {
	tests := []struct {
		name    string
		schema  IndexSchema
		command string
	}{
		{
			name: "simple schema",
			schema: IndexSchema{
				Tags: []Tag{
					{Name: "genre"},
				},
				Texts: []Text{
					{Name: "title"},
				},
				Numerics: []Numeric{
					{Name: "rating"},
				},
				Vectors: []Vector{
					{Name: "embedding"},
				},
			},
			command: "genre TAG SEPARATOR , title TEXT rating NUMERIC embedding VECTOR FLAT 6 TYPE FLOAT32 DIM 128 DISTANCE_METRIC COSINE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.schema.ToCommand()
			if strings.Join(got, " ") != tt.command {
				t.Errorf("IndexSchema.ToCommand() = %v, want %v", got, tt.command)
			}
		})
	}
}

func TestIndexToCommand(t *testing.T) {
	tests := []struct {
		name    string
		index   Index
		command string
	}{
		{
			name: "simple index",
			index: Index{
				Name:   "movies",
				Prefix: []string{"doc:movies"},
				Schema: &IndexSchema{
					Tags: []Tag{
						{Name: "genre"},
					},
					Texts: []Text{
						{Name: "title"},
					},
					Numerics: []Numeric{
						{Name: "rating"},
					},
					Vectors: []Vector{
						{Name: "embedding"},
					},
				},
			},
			command: "FT.CREATE movies ON HASH PREFIX 1 doc:movies SCORE 1.0 SCHEMA genre TAG SEPARATOR , title TEXT rating NUMERIC embedding VECTOR FLAT 6 TYPE FLOAT32 DIM 128 DISTANCE_METRIC COSINE",
		},
		{
			name: "index with all options",
			index: Index{
				Name:      "movies",
				Prefix:    []string{"doc:movies"},
				IndexType: "JSON",
				Schema: &IndexSchema{
					Tags: []Tag{
						{Name: "genre", As: "g", Separator: "|", CaseSensitive: true, Sortable: true, NoIndex: true, IndexMissing: true, IndexEmpty: true},
					},
					Texts: []Text{
						{Name: "title", As: "t", Weight: 1.5, WithSuffixTire: true, Sortable: true, NoIndex: true, Phonetic: "dm:en", IndexMissing: true, IndexEmpty: true},
					},
					Numerics: []Numeric{
						{Name: "rating", As: "r", NoIndex: true, Sortable: true, IndexMissing: true},
					},
					Vectors: []Vector{
						{Name: "embedding", As: "e", Algorithm: "HNSW", Dim: 256, DistanceMetric: "L2", VectorType: "BFLOAT16", M: 16, EfConstruction: 200, EfRuntime: 2000, Epsilon: 0.1},
					},
				},
				Filter:        "@genre:{action} @rating:[1.0 5.0]",
				Language:      "english",
				LanguageField: "title",
				Score:         1.5,
				ScoreField:    "rating",
				MaxTextFields: 10,
				NoOffset:      true,
				NoFields:      true,
			},
			command: "FT.CREATE movies ON JSON PREFIX 1 doc:movies FILTER @genre:{action} @rating:[1.0 5.0] LANGUAGE english LANGUAGE_FIELD title SCORE 1.5 SCORE_FIELD rating MAXTEXTFIELDS 10 NOOFFSET NOFIELDS SCHEMA genre AS g TAG SEPARATOR | CASESENSITIVE SORTABLE NOINDEX INDEXMISSING INDEXEMPTY title AS t TEXT WEIGHT 1.5 WITHSUFFIXTRIE SORTABLE NOINDEX PHONETIC dm:en INDEXMISSING INDEXEMPTY rating AS r NUMERIC SORTABLE INDEXMISSING NOINDEX embedding AS e VECTOR HNSW 14 TYPE BFLOAT16 DIM 256 DISTANCE_METRIC L2 M 16 EF_CONSTRUCTION 200 EF_RUNTIME 2000 EPSILON 0.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.index.ToCommand()
			if got.ToString() != tt.command {
				t.Errorf("Index.ToCommand() = %v, want %v", got.ToString(), tt.command)
			}
		})
	}
}
