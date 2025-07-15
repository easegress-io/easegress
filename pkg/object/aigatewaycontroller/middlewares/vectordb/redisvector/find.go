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
	RedisVectorFind struct {
		index          string
		query          string
		noContent      bool
		verbatim       bool
		withScores     bool
		withSortKeys   bool
		inKeys         []string
		inFields       []string
		returns        []string
		limit          int
		timeout        int
		paramsValues   []float32
		scoreThreshold float32
		offset         int
		sortBy         []string
		paramsKey      string
	}

	Option func(*RedisVectorFind)
)

func NewRedisVectorFind(index, query string, paramsValues []float32, paramsKey string, opts ...Option) *RedisVectorFind {
	find := &RedisVectorFind{
		index:        index,
		query:        query,
		paramsValues: paramsValues,
		paramsKey:    paramsKey,
	}

	for _, opt := range opts {
		opt(find)
	}

	return find
}

func WithNoContent() Option {
	return func(f *RedisVectorFind) {
		f.noContent = true
	}
}

func WithVerbatim() Option {
	return func(f *RedisVectorFind) {
		f.verbatim = true
	}
}

func WithWithScores() Option {
	return func(f *RedisVectorFind) {
		f.withScores = true
	}
}

func WithWithSortKeys() Option {
	return func(f *RedisVectorFind) {
		f.withSortKeys = true
	}
}

func WithInKeys(inKeys []string) Option {
	return func(f *RedisVectorFind) {
		f.inKeys = inKeys
	}
}

func WithInFields(inFields []string) Option {
	return func(f *RedisVectorFind) {
		f.inFields = inFields
	}
}

func WithReturns(returns []string) Option {
	return func(f *RedisVectorFind) {
		f.returns = returns
	}
}

func WithLimit(limit int) Option {
	return func(f *RedisVectorFind) {
		f.limit = limit
	}
}

func WithTimeout(timeout int) Option {
	return func(f *RedisVectorFind) {
		f.timeout = timeout
	}
}

func WithScoreThreshold(scoreThreshold float32) Option {
	return func(f *RedisVectorFind) {
		f.scoreThreshold = scoreThreshold
	}
}

func WithOffset(offset int) Option {
	return func(f *RedisVectorFind) {
		f.offset = offset
	}
}

func WithSortBy(sortBy []string) Option {
	return func(f *RedisVectorFind) {
		f.sortBy = sortBy
	}
}

func (f *RedisVectorFind) ToCommand() *RedisArbitraryCommand {
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

	params := []string{vectorPlaceHolder, float32VectorToString(f.paramsValues)}
	if f.scoreThreshold > 0 && f.scoreThreshold < 1 {
		filter := fmt.Sprintf("@%s:[VECTOR_RANGE $distance_threshold $%s]=>{$YIELD_DISTANCE_AS: %s}", f.paramsKey, vectorPlaceHolder, distancePlaceHolder)
		if f.query != "" {
			filter = fmt.Sprintf("(%s) %s", f.query, filter)
			command.Args = append(command.Args, filter)
			params = append(params, "distance_threshold", strconv.FormatFloat(float64(1.0-f.scoreThreshold), 'f', -1, 32))
		}
	} else {
		filter := "*"
		if f.query != "" {
			filter = f.query
		}
		command.Args = append(command.Args, fmt.Sprintf("(%s)=>[KNN %d @%s $%s AS %s]", filter, f.limit, f.paramsKey, vectorPlaceHolder, distancePlaceHolder))
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
