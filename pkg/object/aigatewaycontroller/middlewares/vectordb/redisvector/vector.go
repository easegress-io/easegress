package redisvector

import (
	"context"
	"fmt"

	"github.com/redis/rueidis"

	"github.com/megaease/easegress/v2/pkg/object/aigatewaycontroller/middlewares/vectordb/vecdbtypes"
)

type (
	// RedisVectorDBSpec defines the specification for a vector database middleware.
	RedisVectorDBSpec struct {
		URL string `json:"url" jsonschema:"required"`
		opt rueidis.ClientOption
	}

	RedisVectorDB struct {
		CommonSpec *vecdbtypes.CommonSpec
		Spec       *RedisVectorDBSpec `json:"spec,omitempty" jsonschema:"required"`
	}

	RedisVectorHandler struct {
		client *RedisClient
		index  string
		schema *IndexSchema
	}
)

// New creates a new NewRedisVectorDB with the given URL.
func New(common *vecdbtypes.CommonSpec, spec *RedisVectorDBSpec) *RedisVectorDB {
	return &RedisVectorDB{
		CommonSpec: common,
		Spec:       spec,
	}
}

func (r *RedisVectorDB) CreateSchema(ctx context.Context, options ...vecdbtypes.Option) (vecdbtypes.VectorHandler, error) {
	clientHandler := &RedisVectorHandler{}
	clientOption, err := rueidis.ParseURL(r.Spec.URL)
	if err != nil {
		return nil, NewErrParsingRedisURL("failed to parse Redis URL", err)
	}

	client, err := NewRedisClient(clientOption)
	if err != nil {
		return nil, NewErrCreateRedisClient("failed to create Redis client", err)
	}

	opts := &vecdbtypes.Options{}
	for _, opt := range options {
		opt(opts)
	}

	clientHandler.client = client
	clientHandler.index = opts.DBName

	if !clientHandler.client.CheckIndexExists(ctx, clientHandler.index) {
		schema, ok := opts.Schema.(*IndexSchema)
		if !ok {
			return nil, NewErrUnexpectedIndexSchema("unexpected index schema type", fmt.Errorf("expected IndexSchema, got %T", opts.Schema))
		}
		clientHandler.schema = schema
		if err := clientHandler.client.CreateIndexIfNotExists(ctx, clientHandler.index, schema); err != nil {
			return nil, NewErrCreateRedisIndex("failed to create index", err)
		}
	}

	return clientHandler, nil
}

func ValidateSpec(spec *RedisVectorDBSpec) error {
	if spec == nil {
		return fmt.Errorf("redis vector spec is nil")
	}
	if spec.URL == "" {
		return fmt.Errorf("redis vector url is empty")
	}
	return nil
}

var _ vecdbtypes.VectorHandler = (*RedisVectorHandler)(nil)

func (r *RedisVectorHandler) SimilaritySearch(ctx context.Context, options ...vecdbtypes.HandlerSearchOption) ([]map[string]any, error) {
	opts := getHandlerSearchOptions(options...)
	if opts.SelectedFields == nil {
		fields := r.schema.GetDefaultSelectedFields()
		opts.SelectedFields = fields
	}
	searchOpts, err := toRedisQueryOptions(*opts)
	if err != nil {
		return nil, err
	}

	query := NewRedisVectorQuery(r.index, opts.RedisFilters, opts.RedisVectorFilterKey, opts.RedisVectorFilterValues, searchOpts...)
	_, docs, err := r.client.Find(ctx, query)
	return docs, err
}

func (r *RedisVectorHandler) InsertDocuments(ctx context.Context, doc []map[string]any, options ...vecdbtypes.HandlerInsertOption) ([]string, error) {
	if doc == nil {
		doc = []map[string]any{}
	}

	opts := getHandlerInsertOptions(options...)

	docIDs, err := r.client.InsertManyWithHash(ctx, opts.RedisPrefix, doc)
	if err != nil {
		return nil, NewErrInsertDocument("failed to insert document", err)
	}
	return docIDs, nil
}

func getHandlerInsertOptions(options ...vecdbtypes.HandlerInsertOption) *vecdbtypes.HandlerInsertOptions {
	opts := &vecdbtypes.HandlerInsertOptions{}
	for _, opt := range options {
		opt(opts)
	}
	return opts
}

func getHandlerSearchOptions(options ...vecdbtypes.HandlerSearchOption) *vecdbtypes.HandlerSearchOptions {
	opts := &vecdbtypes.HandlerSearchOptions{}
	for _, opt := range options {
		opt(opts)
	}
	return opts
}

func toRedisQueryOptions(options vecdbtypes.HandlerSearchOptions) ([]Option, error) {
	opts := []Option{
		WithLimit(options.Limit),
		WithOffset(options.Offset),
		WithSortBy(options.SortBy),
		WithTimeout(options.Timeout),
	}

	if options.ScoreThreshold < 0 || options.ScoreThreshold > 1 {
		return nil, NewInvalidScoreThreshold()
	}

	opts = append(opts, WithScoreThreshold(options.ScoreThreshold))

	if options.RedisNoContent {
		opts = append(opts, WithNoContent())
	}

	if options.RedisVerbatim {
		opts = append(opts, WithVerbatim())
	}

	if options.RedisWithScores {
		opts = append(opts, WithScores())
	}

	if options.RedisWithSortKeys {
		opts = append(opts, WithSortKeys())
	}

	if options.RedisInKeys != nil {
		opts = append(opts, WithInKeys(options.RedisInKeys))
	}

	if options.RedisInFields != nil {
		opts = append(opts, WithInFields(options.RedisInFields))
	}

	if options.SelectedFields != nil {
		opts = append(opts, WithReturns(options.SelectedFields))
	}

	return opts, nil
}
