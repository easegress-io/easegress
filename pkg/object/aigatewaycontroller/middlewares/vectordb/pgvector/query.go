package pgvector

import (
	"github.com/megaease/easegress/v2/pkg/object/aigatewaycontroller/middlewares/vectordb/vecdbtypes"
	"slices"
)

var (
	validDistanceAlgorithms = []string{"<=>", "<->", "<#>", "<+>", "<~>", "<%>"}
)

type (
	PostgresVectorQuery struct {
		tableName         string
		vectorKey         string
		vectorValues      []float32
		distanceAlgorithm string
		filters           string
		limit             int
		offset            int
	}

	Option func(query *PostgresVectorQuery)
)

func NewPostgresVectorQuery(tableName, vectorKey string, vectorValues []float32, opts ...Option) *PostgresVectorQuery {
	query := &PostgresVectorQuery{
		tableName:    tableName,
		vectorKey:    vectorKey,
		vectorValues: vectorValues,
	}

	for _, opt := range opts {
		opt(query)
	}

	if query.distanceAlgorithm == "" || !slices.Contains(validDistanceAlgorithms, query.distanceAlgorithm) {
		query.distanceAlgorithm = "<=>"
	}

	if query.limit <= 0 {
		query.limit = 1
	}

	return query
}

func WithFilters(filters string) Option {
	return func(query *PostgresVectorQuery) {
		query.filters = filters
	}
}

func WithDistanceAlgorithm(algorithm string) Option {
	return func(query *PostgresVectorQuery) {
		if slices.Contains(validDistanceAlgorithms, algorithm) {
			query.distanceAlgorithm = algorithm
		} else {
			query.distanceAlgorithm = "<=>"
		}
	}
}

func WithLimit(limit int) Option {
	return func(query *PostgresVectorQuery) {
		query.limit = limit
	}
}

func WithOffset(offset int) Option {
	return func(query *PostgresVectorQuery) {
		query.offset = offset
	}
}

func toPostgresQueryOptions(options vecdbtypes.HandlerSearchOptions) ([]Option, error) {
	opts := []Option{
		WithLimit(options.Limit),
		WithOffset(options.Offset),
	}

	if options.PostgresDistanceAlgorithm != "" && slices.Contains(validDistanceAlgorithms, options.PostgresDistanceAlgorithm) {
		opts = append(opts, WithDistanceAlgorithm(options.PostgresDistanceAlgorithm))
	} else {
		opts = append(opts, WithDistanceAlgorithm("<=>"))
	}

	if options.PostgresFilters != "" {
		opts = append(opts, WithFilters(options.PostgresFilters))
	}

	return opts, nil
}
