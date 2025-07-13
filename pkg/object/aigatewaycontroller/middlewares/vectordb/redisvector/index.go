package redisvector

import (
	"golang.org/x/exp/slices"
	"strconv"
)

// TODO: This file is a placeholder for the Redis Index(which is equals to the database).
// It should be implemented with the schema operations and search operations.

var (
	validPhoneticMatcherTypes = []PhoneticMatcherType{
		"dm:en", "dm:fr", "dm:pt", "dm:es",
	}

	validVectorDataTypes = []VectorDataType{
		"FLOAT32", "FLOAT64", "BFLOAT16", "FLOAT16",
	}

	validVectorAlgorithms = []VectorAlgorithm{
		"FLAT", "HNSW",
	}

	validDistanceMetrics = []DistanceMetric{
		"L2", "COSINE", "IP",
	}

	validIndexTypes = []IndexType{
		"HASH", "JSON",
	}
)

type (
	PhoneticMatcherType string
	VectorDataType      string
	VectorAlgorithm     string
	DistanceMetric      string
	IndexType           string

	Tag struct {
		Name          string
		As            string
		Separator     string
		CaseSensitive bool
		Sortable      bool
		NoIndex       bool
		IndexMissing  bool
		IndexEmpty    bool
	}

	Text struct {
		Name           string
		As             string
		Weight         float32
		WithSuffixTire bool
		Sortable       bool
		NoIndex        bool
		NoStream       bool
		Phonetic       PhoneticMatcherType
		IndexMissing   bool
		IndexEmpty     bool
	}

	Numeric struct {
		Name         string
		As           string
		Sortable     bool
		IndexMissing bool
		NoIndex      bool
	}

	Vector struct {
		Name           string
		As             string
		Algorithm      VectorAlgorithm
		VectorType     VectorDataType
		Dim            int
		DistanceMetric DistanceMetric

		// only for HNSW algorithm
		M              int
		EfConstruction int
		EfRuntime      int
		Epsilon        float32
	}

	IndexSchema struct {
		Tags     []Tag
		Texts    []Text
		Numerics []Numeric
		Vectors  []Vector
		// TODO: GEO and GEOSHAPE
	}

	RedisArbitraryCommand struct {
		Commands []string
		Keys     []string
		Args     []string
	}

	Index struct {
		Name          string
		IndexType     IndexType
		Prefix        []string
		Filter        string
		Language      string
		LanguageField string
		Score         float32
		ScoreField    string
		MaxTextFields int
		Schema        *IndexSchema
		NoOffset      bool
		NoFields      bool
	}
)

func (t *Tag) ToCommand() []string {
	commands := []string{t.Name}
	if t.As != "" {
		commands = append(commands, "AS", t.As)
	}
	commands = append(commands, "TAG")
	if t.Separator == "" {
		t.Separator = ","
	}
	commands = append(commands, "SEPARATOR", t.Separator)
	if t.CaseSensitive {
		commands = append(commands, "CASESENSITIVE")
	}
	if t.Sortable {
		commands = append(commands, "SORTABLE")
	}
	if t.NoIndex {
		commands = append(commands, "NOINDEX")
	}
	if t.IndexMissing {
		commands = append(commands, "INDEXMISSING")
	}
	if t.IndexEmpty {
		commands = append(commands, "INDEXEMPTY")
	}
	return commands
}

func (t *Text) ToCommand() []string {
	commands := []string{t.Name}
	if t.As != "" {
		commands = append(commands, "AS", t.As)
	}
	commands = append(commands, "TEXT")
	if t.Weight > 0 {
		commands = append(commands, "WEIGHT", strconv.FormatFloat(float64(t.Weight), 'f', -1, 32))
	}
	if t.WithSuffixTire {
		commands = append(commands, "WITHSUFFIXTRIE")
	}
	if t.Sortable {
		commands = append(commands, "SORTABLE")
	}
	if t.NoIndex {
		commands = append(commands, "NOINDEX")
	}
	if t.NoStream {
		commands = append(commands, "NOSTREAM")
	}
	if slices.Contains(validPhoneticMatcherTypes, t.Phonetic) {
		commands = append(commands, "PHONETIC", string(t.Phonetic))
	}
	if t.IndexMissing {
		commands = append(commands, "INDEXMISSING")
	}
	if t.IndexEmpty {
		commands = append(commands, "INDEXEMPTY")
	}
	return commands
}

func (n *Numeric) ToCommand() []string {
	commands := []string{n.Name}
	if n.As != "" {
		commands = append(commands, "AS", n.As)
	}
	commands = append(commands, "NUMERIC")
	if n.Sortable {
		commands = append(commands, "SORTABLE")
	}
	if n.IndexMissing {
		commands = append(commands, "INDEXMISSING")
	}
	if n.NoIndex {
		commands = append(commands, "NOINDEX")
	}
	return commands
}

func (v *Vector) ToCommand() []string {
	commands := []string{v.Name}
	if v.As != "" {
		commands = append(commands, "AS", v.As)
	}
	commands = append(commands, "VECTOR")
	if slices.Contains(validVectorAlgorithms, v.Algorithm) {
		commands = append(commands, "ALGORITHM", string(v.Algorithm))
	} else {
		commands = append(commands, "ALGORITHM", "FLAT")
	}

	args := make([]string, 0)
	if slices.Contains(validVectorDataTypes, v.VectorType) {
		args = append(args, "TYPE", string(v.VectorType))
	} else {
		args = append(args, "TYPE", "FLOAT32")
	}

	if v.Dim > 0 {
		args = append(args, "DIM", strconv.Itoa(v.Dim))
	} else {
		args = append(args, "DIM", "128")
	}

	if slices.Contains(validDistanceMetrics, v.DistanceMetric) {
		args = append(args, "DISTANCE_METRIC", string(v.DistanceMetric))
	} else {
		args = append(args, "DISTANCE_METRIC", "COSINE")
	}

	count := 3
	if v.M > 0 {
		args = append(args, "M", strconv.Itoa(v.M))
		count++
	}
	if v.EfConstruction > 0 {
		args = append(args, "EF_CONSTRUCTION", strconv.Itoa(v.EfConstruction))
		count++
	}
	if v.EfRuntime > 0 {
		args = append(args, "EF_RUNTIME", strconv.Itoa(v.EfRuntime))
		count++
	}
	if v.Epsilon > 0 {
		args = append(args, "EPSILON", strconv.FormatFloat(float64(v.Epsilon), 'f', -1, 32))
	}

	commands = append(commands, strconv.Itoa(count*2))
	commands = append(commands, args...)
	return commands
}

func (s *IndexSchema) ToCommand() []string {
	commands := make([]string, 0)

	if len(s.Tags) > 0 {
		for _, tag := range s.Tags {
			commands = append(commands, tag.ToCommand()...)
		}
	}

	if len(s.Texts) > 0 {
		for _, text := range s.Texts {
			commands = append(commands, text.ToCommand()...)
		}
	}

	if len(s.Numerics) > 0 {
		for _, numeric := range s.Numerics {
			commands = append(commands, numeric.ToCommand()...)
		}
	}

	if len(s.Vectors) > 0 {
		for _, vector := range s.Vectors {
			commands = append(commands, vector.ToCommand()...)
		}
	}

	return commands
}

func (i *Index) ToCommand() *RedisArbitraryCommand {
	command := &RedisArbitraryCommand{
		Commands: []string{"FT.CREATE"},
		Keys:     []string{i.Name},
		Args:     make([]string, 0),
	}

	if i.IndexType != "" {
		if slices.Contains(validIndexTypes, i.IndexType) {
			command.Args = append(command.Args, "ON", string(i.IndexType))
		} else {
			command.Args = append(command.Args, "ON", "HASH")
		}
	}

	if len(i.Prefix) > 0 {
		command.Args = append(command.Args, "PREFIX", strconv.Itoa(len(i.Prefix)))
		command.Args = append(command.Args, i.Prefix...)
	}

	if i.Filter != "" {
		command.Args = append(command.Args, "FILTER", i.Filter)
	}

	if i.Language != "" {
		command.Args = append(command.Args, "LANGUAGE", i.Language)
	}

	if i.LanguageField != "" {
		command.Args = append(command.Args, "LANGUAGE_FIELD", i.LanguageField)
	}

	if i.Score > 0 {
		command.Args = append(command.Args, "SCORE", strconv.FormatFloat(float64(i.Score), 'f', -1, 32))
	} else {
		command.Args = append(command.Args, "SCORE", "1.0")
	}

	if i.ScoreField != "" {
		command.Args = append(command.Args, "SCORE_FIELD", i.ScoreField)
	}

	if i.MaxTextFields > 0 {
		command.Args = append(command.Args, "MAXTEXTFIELDS", strconv.Itoa(i.MaxTextFields))
	}

	if i.NoOffset {
		command.Args = append(command.Args, "NOOFFSET")
	}
	if i.NoFields {
		command.Args = append(command.Args, "NOFIELDS")
	}

	command.Args = append(command.Args, "SCHEMA")
	if i.Schema != nil {
		schemaCommands := i.Schema.ToCommand()
		command.Args = append(command.Args, schemaCommands...)
	}

	return command
}
