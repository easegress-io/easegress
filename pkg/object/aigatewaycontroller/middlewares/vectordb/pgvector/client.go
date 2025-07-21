package pgvector

import (
	"context"
	"github.com/jackc/pgx/v5"
	//"github.com/jackc/pgx/v5/pgtype"
	//"github.com/jackc/pgx/v5/pgxpool"
	//"github.com/pgvector/pgvector-go"
	//pgxvec "github.com/pgvector/pgvector-go/pgx"
)

const (
	EnablePGExtensionLockID = 0x1e2d3c4b5a6b7c8d // Arbitrary lock ID for advisory lock to create extension
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

func (c *PostgresClient) CreateDBIfNotExists(ctx context.Context, tx pgx.Tx) error {
	return nil
}
