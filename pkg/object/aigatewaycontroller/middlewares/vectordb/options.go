package vectordb

import (
	"github.com/megaease/easegress/v2/pkg/object/aigatewaycontroller/middlewares/embeddings"
)

type Option func(*Options)

// Options is the struct for options.
type Options struct {
	// DBName is the name of the vector database.
	DBName string
	// ScoreThreshold is the threshold for the similarity score.
	ScoreThreshold float32
	// Filters is the metadata filters to apply.
	Filters interface{}
	// EmbedHandler is the handler to use for embedding documents.
	EmbedHandler embeddings.EmbeddingHandler
}

// WithDBName returns an Option for setting the vector database name.
func WithDBName(dbName string) Option {
	return func(o *Options) {
		o.DBName = dbName
	}
}

// WithScoreThreshold returns an Option for setting the score threshold.
func WithScoreThreshold(scoreThreshold float32) Option {
	return func(o *Options) {
		o.ScoreThreshold = scoreThreshold
	}
}

// WithFilters returns an Option for setting the metadata filters.
func WithFilters(filters interface{}) Option {
	return func(o *Options) {
		o.Filters = filters
	}
}

// WithEmbedHandler returns an Option for setting the embedding handler.
func WithEmbedHandler(embedHandler embeddings.EmbeddingHandler) Option {
	return func(o *Options) {
		o.EmbedHandler = embedHandler
	}
}
