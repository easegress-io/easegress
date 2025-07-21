package pgvector

import (
	"context"

	"github.com/megaease/easegress/v2/pkg/object/aigatewaycontroller/middlewares/vectordb/vecdbtypes"
)

type (
	// PostgresVectorDBSpec defines the specification for a Postgres vector database middleware.
	PostgresVectorDBSpec struct {
		ConnectionURL string `json:"connectionURL" jsonschema:"required"`
	}

	PostgresVectorDB struct {
		CommonSpec *vecdbtypes.CommonSpec `json:"commonSpec,omitempty" jsonschema:"required"`
		Spec       *PostgresVectorDBSpec  `json:"spec,omitempty" jsonschema:"required"`
	}

	PostgresVectorHandler struct {
		client *PostgresClient
		DBName string
		schema *TableSchema
	}
)

// New creates a new PostgresVectorDB with the given connection URL.
func New(common *vecdbtypes.CommonSpec, spec *PostgresVectorDBSpec) *PostgresVectorDB {
	return &PostgresVectorDB{
		CommonSpec: common,
		Spec:       spec,
	}
}

func (p *PostgresVectorDB) CreateSchema(ctx context.Context, options ...vecdbtypes.Option) (vecdbtypes.VectorHandler, error) {
	clientHandler := &PostgresVectorHandler{}
	client, err := NewPostgresClient(ctx, p.Spec.ConnectionURL)
	if err != nil {
		return nil, NewErrCreatePostgresClient("failed to create Postgres client", err)
	}

	tx, err := client.conn.Begin(ctx)
	if err != nil {
		return nil, NewErrBeginTransaction("failed to begin transaction", err)
	}
	err = client.EnableVectorExtensionIfNotExists(ctx, tx)
	if err != nil {
		_ = tx.Rollback(ctx)
		return nil, NewErrEnableVectorExtension("failed to enable vector extension", err)
	}

	opts := &vecdbtypes.Options{}
	for _, opt := range options {
		opt(opts)
	}

	clientHandler.client = client
	clientHandler.DBName = opts.DBName

	if !client.CheckDBExists(ctx, opts.DBName) {
		schema, ok := opts.Schema.(*TableSchema)
		if !ok {
			return nil, NewErrUnexpectedSchemaType("unexpected schema type, expected TableSchema", err)
		}
		clientHandler.schema = schema
		if err := client.CreateDBIfNotExists(ctx, tx); err != nil {
			_ = tx.Rollback(ctx)
			return nil, NewErrCreatePostgresDB("failed to create Postgres database", err)
		}
	}

	return clientHandler, tx.Commit(ctx)
}

var _ vecdbtypes.VectorHandler = (*PostgresVectorHandler)(nil)

func (p PostgresVectorHandler) SimilaritySearch(ctx context.Context, options ...vecdbtypes.HandlerSearchOption) ([]map[string]any, error) {
	//TODO implement me
	panic("implement me")
}

func (p PostgresVectorHandler) InsertDocuments(ctx context.Context, doc []map[string]any, options ...vecdbtypes.HandlerInsertOption) ([]string, error) {
	//TODO implement me
	panic("implement me")
}
