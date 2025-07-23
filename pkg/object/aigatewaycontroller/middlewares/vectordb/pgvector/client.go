package pgvector

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/pgvector/pgvector-go"
	"slices"
	//"github.com/jackc/pgx/v5/pgtype"
	//"github.com/jackc/pgx/v5/pgxpool"
	//"github.com/pgvector/pgvector-go"
	pgxvec "github.com/pgvector/pgvector-go/pgx"
)

const (
	DefaultPrimaryKeyColumnName = "id"               // Default primary key column name
	EnablePGExtensionLockID     = 0x1e2d3c4b5a6b7c8d // Arbitrary lock ID for advisory lock to create extension
)

type (
	PostgresClient struct {
		conn *pgx.Conn
	}
)

// NewPostgresClient creates a new Postgres client with the given connection URL.
func NewPostgresClient(ctx context.Context, connectionURL string) (*PostgresClient, error) {
	conn, err := pgx.Connect(ctx, connectionURL)
	if err != nil {
		return nil, err
	}
	err = pgxvec.RegisterTypes(ctx, conn)
	if err != nil {
		_ = conn.Close(ctx)
		return nil, fmt.Errorf("failed to register pgvector types: %w", err)
	}
	return &PostgresClient{conn: conn}, nil
}

func (c *PostgresClient) EnableVectorExtensionIfNotExists(ctx context.Context, tx pgx.Tx) error {
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", EnablePGExtensionLockID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS vector"); err != nil {
		return err
	}
	return nil
}

func (c *PostgresClient) CheckDBExists(ctx context.Context, dbName string) bool {
	var exists bool
	err := c.conn.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)", dbName).Scan(&exists)
	if err != nil {
		return false
	}
	return exists
}

func (c *PostgresClient) CreateDBIfNotExists(ctx context.Context, tx pgx.Tx, schema *TableSchema) error {
	if schema == nil || schema.TableName == "" {
		return fmt.Errorf("invalid schema: %v", schema)
	}

	// Check if the schema has an "id" column
	idExists := false
	for _, col := range schema.Columns {
		if col.Name == DefaultPrimaryKeyColumnName {
			idExists = true
			break
		}
	}

	if !idExists {
		schema.Columns = append(schema.Columns, Column{
			Name:       DefaultPrimaryKeyColumnName,
			DataType:   "uuid",
			IsPrimary:  true,
			IsNullable: false,
		})
	}

	sql := "CREATE TABLE IF NOT EXISTS %s ("
	for i, col := range schema.Columns {
		sql += fmt.Sprintf("%s %s ", col.Name, col.DataType)
		if col.IsNullable {
			sql += "NULL "
		} else {
			sql += "NOT NULL "
		}

		if col.IsUnique {
			sql += fmt.Sprintf("UNIQUE ")
		}

		if col.IsPrimary && col.Name == DefaultPrimaryKeyColumnName {
			sql += "PRIMARY KEY "
		}

		if col.DefaultValue != "" {
			sql += fmt.Sprintf("DEFAULT %s", col.DefaultValue)
		}
		if i == len(schema.Columns)-1 {
			sql += fmt.Sprintf(");")
		} else {
			sql += ", "
		}
	}

	_, err := tx.Exec(ctx, fmt.Sprintf(sql, schema.TableName))
	if err != nil {
		return fmt.Errorf("failed to create table %s: %w", schema.TableName, err)
	}

	// Create indexes if specified
	for _, index := range schema.Indexes {
		indexSQL := "CREATE INDEX IF NOT EXISTS %s ON %s USING %s ("
		if index.Type == IndexTypeHNSW {
			indexSQL += "%s %s)"
			indexSQL = fmt.Sprintf(indexSQL, index.Name, schema.TableName, index.Type, index.Column, index.HNSW.DistanceMetric)

			if index.HNSW.M != 0 || index.HNSW.EfConstruction != 0 {
				if index.HNSW.DistanceMetric == "" {
					return fmt.Errorf("distance metric is required for HNSW index")
				}
				if !slices.Contains(validHNSWDistanceMetrics, index.HNSW.DistanceMetric) {
					return fmt.Errorf("invalid distance metric %s for HNSW index", index.HNSW.DistanceMetric)
				}
				if index.HNSW.M != 0 {
					indexSQL += fmt.Sprintf(" WITH (m = %d", index.HNSW.M)
				} else {
					indexSQL += " WITH ("
				}
				if index.HNSW.EfConstruction != 0 {
					indexSQL += fmt.Sprintf(", ef_construction = %d", index.HNSW.EfConstruction)
				}
				indexSQL += ");"
			} else {
				indexSQL += ");"
			}
		} else if index.Type == IndexTypeIVFFlat {
			if index.IVFFlat.DistanceMetric == "" {
				return fmt.Errorf("distance metric is required for IVFFlat index")
			}
			if !slices.Contains(validIVFFlatDistanceMetrics, index.IVFFlat.DistanceMetric) {
				return fmt.Errorf("invalid distance metric %s for IVFFlat index", index.IVFFlat.DistanceMetric)
			}
			indexSQL += "%s %s)"
			indexSQL = fmt.Sprintf(indexSQL, index.Name, schema.TableName, index.Type, index.Column, index.IVFFlat.DistanceMetric)
			if index.IVFFlat.Nlist != 0 {
				indexSQL += fmt.Sprintf(" WITH (lists = %d);", index.IVFFlat.Nlist)
			} else {
				indexSQL += ");"
			}
		} else {
			indexSQL = fmt.Sprintf("CREATE INDEX IF NOT EXISTS %s ON %s USING %s (%s);",
				index.Name, schema.TableName, index.Type, index.Column)
		}
		if _, err := tx.Exec(ctx, indexSQL); err != nil {
			return fmt.Errorf("failed to create index %s on table %s: %w", index.Name, schema.TableName, err)
		}
	}

	return err
}

func (c *PostgresClient) InsertWithVector(ctx context.Context, tableName string, doc []map[string]any) ([]string, error) {
	if len(doc) == 0 {
		return []string{}, nil
	}

	var docIDs []string
	b := &pgx.Batch{}
	for _, d := range doc {
		if _, ok := d[DefaultPrimaryKeyColumnName]; !ok || d[DefaultPrimaryKeyColumnName] == nil {
			d[DefaultPrimaryKeyColumnName] = uuid.New().String()
			docIDs = append(docIDs, d[DefaultPrimaryKeyColumnName].(string))
		} else {
			docIDs = append(docIDs, d[DefaultPrimaryKeyColumnName].(string))
		}

		sql, args, err := c.insertSingleDocument(ctx, tableName, d)
		if err != nil {
			return nil, fmt.Errorf("failed to insert document: %w", err)
		}
		b.Queue(sql, args...)
	}
	return docIDs, c.conn.SendBatch(ctx, b).Close()
}

func (c *PostgresClient) insertSingleDocument(ctx context.Context, tableName string, doc map[string]any) (string, []any, error) {
	if tableName == "" || doc == nil || len(doc) == 0 {
		return "", nil, fmt.Errorf("invalid table name or document")
	}

	sql := fmt.Sprintf("INSERT INTO %s (", tableName)
	index := 0
	for colName := range doc {
		sql += fmt.Sprintf("%s", colName)
		if index < len(doc)-1 {
			sql += ", "
		}
		index++
	}
	sql += ") VALUES ("
	index = 0
	for range doc {
		sql += fmt.Sprintf("$%d", index+1)
		if index < len(doc)-1 {
			sql += ", "
		}
	}
	sql += ");"

	args := make([]any, 0, len(doc))
	for _, value := range doc {
		if vec, ok := value.([]float32); ok {
			value = pgvector.NewVector(vec)
		}
		args = append(args, value)
	}

	return sql, args, nil
}

func (c *PostgresClient) Query(ctx context.Context, query *PostgresVectorQuery) (int64, []map[string]any, error) {
	if query == nil || query.tableName == "" {
		return 0, nil, fmt.Errorf("invalid query: %v", query)
	}

	dims := len(query.vectorValues)
	sql := fmt.Sprintf("SELECT *, (1-(%s%s$1)) AS score FROM %s WHERE vector_dims(%s) = $2", query.vectorKey, query.distanceAlgorithm, query.tableName, query.vectorKey)
	if query.filters != "" {
		sql += fmt.Sprintf(" AND %s", query.filters)
	}
	sql += fmt.Sprintf(" ORDER BY score DESC LIMIT %d", query.limit)
	if query.offset > 0 {
		sql += fmt.Sprintf(" OFFSET %d", query.offset)
	}

	rows, err := c.conn.Query(ctx, sql, pgvector.NewVector(query.vectorValues), dims)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to execute query: %w", err)
	}

	defer rows.Close()

	var docs []map[string]any
	for rows.Next() {
		doc := make(map[string]any)
		// Scan the row into the map
		columns, err := rows.Values()
		if err != nil {
			return 0, nil, fmt.Errorf("failed to get row values: %w", err)
		}
		for i, col := range columns {
			colName := rows.FieldDescriptions()[i].Name
			if colName == "score" {
				doc["score"] = col
			} else {
				if vec, ok := col.(pgvector.Vector); ok {
					doc[colName] = vec.Slice()
				} else {
					doc[colName] = col
				}
			}
		}
		docs = append(docs, doc)
	}

	if err := rows.Err(); err != nil {
		return 0, nil, fmt.Errorf("error iterating over rows: %w", err)
	}

	total := int64(len(docs))
	return total, docs, nil
}
