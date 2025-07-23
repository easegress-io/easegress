package pgvector

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestCreateTableSQL(t *testing.T) {
	tests := []struct {
		name     string
		schema   *TableSchema
		expected string
	}{
		{
			name: "Simple table",
			schema: &TableSchema{
				TableName: "test_table",
				Columns: []Column{
					{Name: "name", DataType: "text"},
					{Name: "vector", DataType: "vector(3)"},
				},
			},
			expected: "CREATE TABLE IF NOT EXISTS %s (name text, vector vector(3), id uuid PRIMARY KEY);",
		},
		{
			name: "Table with primary key and unique constraint",
			schema: &TableSchema{
				TableName: "user_table",
				Columns: []Column{
					{Name: "id", DataType: "uuid", IsPrimary: true, IsNullable: false},
					{Name: "email", DataType: "text", IsUnique: true, IsNullable: true},
					{Name: "name", DataType: "text", IsNullable: true},
					{Name: "vector", DataType: "bit(128)"},
				},
			},
			expected: "CREATE TABLE IF NOT EXISTS %s (id uuid PRIMARY KEY, email text NULL UNIQUE, name text NULL, vector bit(128));",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, err := getCreateTableSQL(tt.schema)
			if err != nil {
				t.Fatalf("getCreateTableSQL() error = %v", err)
			}
			if sql != tt.expected {
				t.Errorf("getCreateTableSQL() = %v, want %v", sql, tt.expected)
			}
		})
	}
}

func TestGetCreateTableIndexSQL(t *testing.T) {
	tests := []struct {
		name     string
		schema   *TableSchema
		index    Index
		expected string
	}{
		{
			name: "HNSW index",
			schema: &TableSchema{
				TableName: "test_table",
			},
			index: Index{
				Name:   "hnsw_index",
				Type:   IndexTypeHNSW,
				Column: "vector",
				HNSW: &HNSWIndexOption{
					DistanceMetric: "vector_l2_ops",
					M:              16,
					EfConstruction: 64,
				},
			},
			expected: "CREATE INDEX IF NOT EXISTS hnsw_index ON test_table USING HNSW (vector vector_l2_ops) WITH (m = 16, ef_construction = 64);",
		},
		{
			name: "IVFFlat index",
			schema: &TableSchema{
				TableName: "test_table",
			},
			index: Index{
				Name:   "ivfflat_index",
				Type:   IndexTypeIVFFlat,
				Column: "vector",
				IVFFlat: &IVFFlatIndexOption{
					DistanceMetric: "vector_l2_ops",
					Nlist:          100,
				},
			},
			expected: "CREATE INDEX IF NOT EXISTS ivfflat_index ON test_table USING IVFFlat (vector vector_l2_ops) WITH (lists = 100);",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, err := getCreateTableIndexSQL(tt.schema, tt.index)
			if err != nil {
				t.Fatalf("getCreateTableIndexSQL() error = %v", err)
			}
			if sql != tt.expected {
				t.Errorf("getCreateTableIndexSQL() = %v, want %v", sql, tt.expected)
			}
		})
	}
}

func TestQuerySQL(t *testing.T) {
	tests := []struct {
		name     string
		query    *PostgresVectorQuery
		expected string
	}{
		{
			name: "Simple query",
			query: &PostgresVectorQuery{
				tableName:         "test_table",
				vectorKey:         "embedding",
				vectorValues:      []float32{0.1, 0.2, 0.3},
				distanceAlgorithm: "<=>",
				limit:             10,
				filters:           "name = 'test'",
			},
			expected: "SELECT *, (1-(embedding<=>$1)) AS score FROM test_table WHERE vector_dims(embedding) = $2 AND name = 'test' ORDER BY score DESC LIMIT 10;",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, err := getQuerySQL(tt.query)
			if err != nil {
				t.Fatalf("QuerySQL() error = %v", err)
			}
			if sql != tt.expected {
				t.Errorf("QuerySQL() = %v, want %v", sql, tt.expected)
			}
		})
	}
}

func TestPostgresClient(t *testing.T) {
	ctx := context.Background()
	req := testcontainers.ContainerRequest{
		Image:        "pgvector/pgvector:pg17",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "postgres",
			"POSTGRES_PASSWORD": "postgres",
		},
		WaitingFor: wait.ForListeningPort("5432/tcp"),
	}

	postgresContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("Failed to start Postgres container: %v", err)
	}
	defer testcontainers.CleanupContainer(t, postgresContainer)
	host, err := postgresContainer.Host(ctx)
	if err != nil {
		t.Fatalf("Failed to get Postgres container host: %v", err)
	}
	port, err := postgresContainer.MappedPort(ctx, "5432")
	if err != nil {
		t.Fatalf("Failed to get Postgres container port: %v", err)
	}

	connectionString := fmt.Sprintf("postgres://postgres:postgres@%s:%s/postgres?sslmode=disable", host, port.Port())
	client, err := NewPostgresClient(ctx, connectionString)
	if err != nil {
		t.Fatalf("Failed to create Postgres client: %v", err)
	}
	defer client.Close(ctx)

	tx, err := client.conn.Begin(ctx)
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}

	err = client.EnableVectorExtensionIfNotExists(ctx, tx)
	if err != nil {
		t.Fatalf("Failed to enable vector extension: %v", err)
	}

	testDBExists := client.CheckDBExists(ctx, "test_db")
	assert.False(t, testDBExists, "Database should not exist before creation")
	schema := &TableSchema{
		TableName: "test_table",
		Columns: []Column{
			{Name: "name", DataType: "text"},
			{Name: "vector", DataType: "vector(3)"},
		},
		Indexes: []Index{
			{
				Name:   "hnsw_index",
				Type:   IndexTypeHNSW,
				Column: "vector",
				HNSW: &HNSWIndexOption{
					DistanceMetric: "vector_l2_ops",
					M:              16,
					EfConstruction: 64,
				},
			},
			{
				Name:   "name_index",
				Type:   IndexTypeHash,
				Column: "name",
			},
		},
	}
	err = client.CreateDBIfNotExists(ctx, tx, schema)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	err = tx.Commit(ctx)
	if err != nil {
		t.Fatalf("Failed to commit transaction: %v", err)
	}

	res, err := client.InsertWithVector(ctx, "test_table", []map[string]any{
		{"name": "test1", "vector": []float32{0.1, 0.2, 0.3}},
		{"name": "test2", "vector": []float32{0.4, 0.5, 0.6}},
	})
	if err != nil {
		t.Fatalf("Failed to insert documents: %v", err)
	}
	assert.Len(t, res, 2, "Expected 2 documents to be inserted")

	query := &PostgresVectorQuery{
		tableName:         "test_table",
		vectorKey:         "vector",
		vectorValues:      []float32{0.1, 0.2, 0.3},
		distanceAlgorithm: "<=>",
		limit:             10,
		filters:           "name = 'test1'",
	}
	n, result, err := client.Query(ctx, query)
	if err != nil {
		t.Fatalf("Failed to query documents: %v", err)
	}
	assert.Equal(t, int64(1), n, "Expected 1 document to match the query")
	assert.Len(t, result, 1, "Expected 1 document in the result")

	query = &PostgresVectorQuery{
		tableName:         "test_table",
		vectorKey:         "vector",
		vectorValues:      []float32{0.1, 0.2, 0.3},
		distanceAlgorithm: "<=>",
		limit:             10,
	}
	n, result, err = client.Query(ctx, query)
	if err != nil {
		t.Fatalf("Failed to query documents: %v", err)
	}
	assert.Equal(t, n, int64(2), "Expected some documents to match the query")
	assert.Equal(t, len(result), 2, "Expected some documents in the result")
}
