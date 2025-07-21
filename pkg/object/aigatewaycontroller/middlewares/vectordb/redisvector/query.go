package redisvector

import (
	"fmt"
	"strconv"
)

const (
	vectorPlaceHolder   = "vector"
	distancePlaceHolder = "distance"
)

type (
	RedisVectorQuery struct {
		index              string
		filters            string
		vectorFilterValues []float32
		vectorFilterKey    string
		noContent          bool
		verbatim           bool
		withScores         bool
		withSortKeys       bool
		inKeys             []string
		inFields           []string
		returns            []string
		limit              int
		timeout            int
		scoreThreshold     float32
		offset             int
		sortBy             []string
	}

	Option func(*RedisVectorQuery)
)

func NewRedisVectorQuery(index, filters string, vectorFilterKey string, vectorFilterValues []float32, opts ...Option) *RedisVectorQuery {
	find := &RedisVectorQuery{
		index:              index,
		filters:            filters,
		vectorFilterValues: vectorFilterValues,
		vectorFilterKey:    vectorFilterKey,
	}

	for _, opt := range opts {
		opt(find)
	}

	return find
}

func WithNoContent() Option {
	return func(f *RedisVectorQuery) {
		f.noContent = true
	}
}

func WithVerbatim() Option {
	return func(f *RedisVectorQuery) {
		f.verbatim = true
	}
}

func WithScores() Option {
	return func(f *RedisVectorQuery) {
		f.withScores = true
	}
}

func WithSortKeys() Option {
	return func(f *RedisVectorQuery) {
		f.withSortKeys = true
	}
}

func WithInKeys(inKeys []string) Option {
	return func(f *RedisVectorQuery) {
		f.inKeys = inKeys
	}
}

func WithInFields(inFields []string) Option {
	return func(f *RedisVectorQuery) {
		f.inFields = inFields
	}
}

func WithReturns(returns []string) Option {
	return func(f *RedisVectorQuery) {
		f.returns = returns
	}
}

func WithLimit(limit int) Option {
	return func(f *RedisVectorQuery) {
		f.limit = limit
	}
}

func WithTimeout(timeout int) Option {
	return func(f *RedisVectorQuery) {
		f.timeout = timeout
	}
}

func WithScoreThreshold(scoreThreshold float32) Option {
	return func(f *RedisVectorQuery) {
		f.scoreThreshold = scoreThreshold
	}
}

func WithOffset(offset int) Option {
	return func(f *RedisVectorQuery) {
		f.offset = offset
	}
}

func WithSortBy(sortBy []string) Option {
	return func(f *RedisVectorQuery) {
		f.sortBy = sortBy
	}
}

func (f *RedisVectorQuery) ToCommand() *RedisArbitraryCommand {
	command := &RedisArbitraryCommand{
		Commands: []string{"FT.SEARCH"},
		Keys:     []string{f.index},
	}
	if f.returns == nil {
		f.returns = []string{}
	}

	if f.limit == 0 {
		f.limit = 1
	}

	params := []string{vectorPlaceHolder, float32VectorToString(f.vectorFilterValues)}
	if f.scoreThreshold > 0 && f.scoreThreshold < 1 {
		filter := fmt.Sprintf("@%s:[VECTOR_RANGE $distance_threshold $%s]=>{$YIELD_DISTANCE_AS: %s}", f.vectorFilterKey, vectorPlaceHolder, distancePlaceHolder)
		if f.filters != "" {
			filter = fmt.Sprintf("\"%s %s\"", f.filters, filter)
			command.Args = append(command.Args, filter)
			params = append(params, "distance_threshold", strconv.FormatFloat(float64(1.0-f.scoreThreshold), 'f', -1, 32))
		}
	} else {
		filter := "*"
		if f.filters != "" {
			filter = f.filters
		}
		command.Args = append(command.Args, fmt.Sprintf("(%s)=>[KNN %d @%s $%s AS %s]", filter, f.limit, f.vectorFilterKey, vectorPlaceHolder, distancePlaceHolder))
	}

	if l := len(f.returns); l > 0 {
		f.returns = append(f.returns, distancePlaceHolder)
		command.Args = append(command.Args, "RETURN", strconv.Itoa(len(f.returns)))
		command.Args = append(command.Args, f.returns...)
	}

	command.Args = append(command.Args, "SORTBY")
	if len(f.sortBy) == 0 {
		f.sortBy = []string{distancePlaceHolder, "ASC"}
	}
	command.Args = append(command.Args, f.sortBy...)

	command.Args = append(command.Args, "DIALECT", "2")
	if f.offset < 0 {
		f.offset = 0
	}
	command.Args = append(command.Args, "LIMIT", strconv.Itoa(f.offset), strconv.Itoa(f.limit))

	command.Args = append(command.Args, "PARAMS", strconv.Itoa(len(params)))
	command.Args = append(command.Args, params...)
	if f.noContent {
		command.Args = append(command.Args, "NO_CONTENT")
	}
	if f.verbatim {
		command.Args = append(command.Args, "VERBATIM")
	}
	if f.withScores {
		command.Args = append(command.Args, "WITHSCORES")
	}
	if f.withSortKeys {
		command.Args = append(command.Args, "WITHSORTKEYS")
	}
	if len(f.inKeys) > 0 {
		command.Args = append(command.Args, "INKEYS", strconv.Itoa(len(f.inKeys)))
		command.Args = append(command.Args, f.inKeys...)
	}
	if len(f.inFields) > 0 {
		command.Args = append(command.Args, "INFIELDS", strconv.Itoa(len(f.inFields)))
		command.Args = append(command.Args, f.inFields...)
	}
	return command
}
