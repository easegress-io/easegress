package redisvector

import "testing"

func TestQueryToCommand(t *testing.T) {
	vector := []float32{0.1, 0.2, 0.3}
	vectorValue := float32VectorToString(vector)
	tests := []struct {
		name    string
		query   *RedisVectorQuery
		command string
	}{
		{
			name:    "simple query",
			query:   NewRedisVectorQuery("books-idx", "", "title_embedding", vector),
			command: "FT.SEARCH books-idx (*)=>[KNN 1 @title_embedding $vector AS distance] SORTBY distance ASC DIALECT 2 LIMIT 0 1 PARAMS 2 vector " + vectorValue,
		},
		{
			name:    "query with filters",
			query:   NewRedisVectorQuery("books-idx", "@genre{fiction}", "title_embedding", vector, WithNoContent(), WithVerbatim(), WithScores(), WithSortBy([]string{"title", "DESC"}), WithSortKeys(), WithInKeys([]string{"book_id"}), WithInFields([]string{"title", "author"}), WithReturns([]string{"title", "author"}), WithOffset(5), WithLimit(10), WithScoreThreshold(0.7)),
			command: "FT.SEARCH books-idx (@genre{fiction}) @title_embedding:[VECTOR_RANGE $distance_threshold $vector]=>{$YIELD_DISTANCE_AS: distance} RETURN 3 title author distance SORTBY title DESC DIALECT 2 LIMIT 5 10 PARAMS 4 vector " + vectorValue + " distance_threshold 0.3 NO_CONTENT VERBATIM WITHSCORES WITHSORTKEYS INKEYS 1 book_id INFIELDS 2 title author",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.query.ToCommand().ToString()
			if got != tt.command {
				t.Errorf("RedisVectorQuery.ToCommand() = %v, want %v", got, tt.command)
			}
		})
	}
}
