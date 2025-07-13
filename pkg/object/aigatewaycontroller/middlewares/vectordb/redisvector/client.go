package redisvector

import (
	"context"
	"errors"
	"fmt"
	"github.com/redis/rueidis"
)

// TODO: This file is a placeholder for the Redis Client.
// It should be a wrapper for the Redis client library.
// It should create a Redis client and provide methods to interact with Redis.
// It should be a higher-level API to call schema and search operations.
// It should have methods to create, drop, and check index existence; add, (delete?) documents; and search documents.

type (
	RedisClient struct {
		client rueidis.Client
	}
)

// NewRedisClient creates a new Redis client with the given options.
func NewRedisClient(opt rueidis.ClientOption) (*RedisClient, error) {
	client, err := rueidis.NewClient(opt)
	if err != nil {
		return nil, err
	}
	return &RedisClient{client: client}, nil
}

// DropIndex drops the index with the given name.
func (c *RedisClient) DropIndex(ctx context.Context, index string, deleteDocuments bool) error {
	if deleteDocuments {
		return c.client.Do(ctx, c.client.B().FtDropindex().Index(index).Dd().Build()).Error()
	}
	return c.client.Do(ctx, c.client.B().FtDropindex().Index(index).Build()).Error()
}

// CheckIndexExists checks if the index with the given name exists.
func (c *RedisClient) CheckIndexExists(ctx context.Context, index string) bool {
	if index == "" {
		return false
	}
	return c.client.Do(ctx, c.client.B().FtInfo().Index(index).Build()).Error() == nil
}

// CreateIndexIfNotExists creates the index with the given name if it does not exist.
func (c *RedisClient) CreateIndexIfNotExists(ctx context.Context, index string, schema *IndexSchema) error {
	if index == "" {
		return errors.New("empty index name")
	}

	if c.CheckIndexExists(ctx, index) {
		return nil
	}

	redisIndex := &Index{
		Name:      index,
		Schema:    schema,
		Prefix:    []string{getPrefix(index)},
		IndexType: "HASH",
	}

	command := redisIndex.ToCommand()
	return c.client.Do(ctx, c.client.B().Arbitrary(command.Commands[0]).Keys(command.Keys[0]).Args(command.Args...).Build()).Error()
}

// getPrefix get prefix with index name.
func getPrefix(index string) string {
	return fmt.Sprintf("doc:%s", index)
}
