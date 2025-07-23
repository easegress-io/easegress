package pgvector

import "testing"

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
