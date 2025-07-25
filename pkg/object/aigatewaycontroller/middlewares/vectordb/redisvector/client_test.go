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
	"context"
	"testing"

	"github.com/redis/rueidis"
	"github.com/stretchr/testify/assert"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestToHmsetCommands(t *testing.T) {
	t.Parallel()

	vector := []float32{0.1, 0.2, 0.3}

	data := map[string]any{
		"content":        "foo",
		"content_vector": vector,
		"tag":            []int{1, 2},
	}

	result := toHmsetCommand("test-prefix", data)
	if len(result.Args) != 6 {
		t.Errorf("Expected 6 args, got %d", len(result.Args))
	}
}

func TestRedisClientIndexOperations(t *testing.T) {
	ctx := context.Background()
	req := testcontainers.ContainerRequest{
		Image:        "redis:latest",
		ExposedPorts: []string{"6379/tcp"},
		WaitingFor:   wait.ForLog("Ready to accept connections"),
	}
	redisC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("Failed to create Redis container: %v", err)
	}
	defer testcontainers.CleanupContainer(t, redisC)
	host, err := redisC.Host(ctx)
	if err != nil {
		t.Fatalf("Failed to get Redis container host: %v", err)
	}
	port, err := redisC.MappedPort(ctx, "6379/tcp")
	if err != nil {
		t.Fatalf("Failed to get Redis container port: %v", err)
	}
	client, err := NewRedisClient(rueidis.ClientOption{
		InitAddress: []string{host + ":" + port.Port()},
	})
	if err != nil {
		t.Fatalf("Failed to create Redis client: %v", err)
	}
	// check index not exists
	assert.False(t, client.CheckIndexExists(ctx, "test_index"))
	// create index
	err = client.CreateIndexIfNotExists(ctx, "movie", &IndexSchema{
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
			{Name: "embedding", Dim: 3},
		},
	})
	if err != nil {
		t.Fatalf("Failed to create index: %v", err)
	}
	// check index exists
	assert.True(t, client.CheckIndexExists(ctx, "movie"))
	// insert data
	results, err := client.InsertManyWithHash(ctx, "movie", []map[string]any{
		{
			"title":     "Inception",
			"genre":     "Sci-Fi",
			"rating":    8.8,
			"embedding": []float32{0.1, 0.2, 0.3},
		},
		{
			"title":     "The Matrix",
			"genre":     "Sci-Fi",
			"rating":    8.7,
			"embedding": []float32{0.2, 0.3, 0.4},
		},
		{
			"title":     "Interstellar",
			"genre":     "Sci-Fi",
			"rating":    8.6,
			"embedding": []float32{0.3, 0.4, 0.5},
		},
		{
			"title":     "The Dark Knight",
			"genre":     "Action",
			"rating":    9.0,
			"embedding": []float32{0.4, 0.5, 0.6},
		},
		{
			"title":     "Pulp Fiction",
			"genre":     "Crime",
			"rating":    8.9,
			"embedding": []float32{0.5, 0.6, 0.7},
		},
	})
	if err != nil {
		t.Fatalf("Failed to insert data: %v", err)
	}
	assert.Len(t, results, 5)
	// search data
	n, movies, err := client.Find(ctx, NewRedisVectorQuery("movie", "@genre:{Crime}", "embedding",
		[]float32{0.1, 0.2, 0.3}, WithLimit(3)))
	if err != nil {
		t.Fatalf("Failed to search data: %v", err)
	}
	assert.Equal(t, int64(1), n)
	for _, movie := range movies {
		assert.Contains(t, []string{"Pulp Fiction"}, movie["title"].(string))
	}
	// drop index
	err = client.DropIndex(ctx, "movie", true)
	if err != nil {
		t.Fatalf("Failed to drop index: %v", err)
	}
}
