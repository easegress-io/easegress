package redisvector

import (
	"context"
	"testing"

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

func TestRedisClient(t *testing.T) {
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
	// Testing code for RedisClient.
}
