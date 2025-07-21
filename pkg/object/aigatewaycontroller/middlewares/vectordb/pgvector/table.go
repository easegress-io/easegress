package pgvector

const (
	// IndexTypeHash represents a hash index type.
	IndexTypeHash IndexType = "HASH"
	// IndexTypeJSON represents a JSON index type.
	IndexTypeJSON IndexType = "JSON"
	// IndexTypeHNSW represents a HNSW index type.
	IndexTypeHNSW IndexType = "HNSW"
	// IndexTypeIVFFlat represents a IVFFlat index type.
	IndexTypeIVFFlat IndexType = "IVFFlat"
)

var (
	validHNSWDistanceMetrics = []string{
		"vector_l2_ops",     // L2 distance
		"vector_ip_ops",     // Inner product
		"vector_cosine_ops", // Cosine distance
		"vector_l1_ops",     // L1 distance
		"bit_hamming_ops",   // Hamming distance
		"bit_jaccard_ops",   // Jaccard distance
	}

	validIVFFlatDistanceMetrics = []string{
		"vector_l2_ops",     // L2 distance
		"vector_ip_ops",     // Inner product
		"vector_cosine_ops", // Cosine distance
		"bit_hamming_ops",   // Hamming distance
	}
)

type (
	TableSchema struct {
		TableName string   `json:"tableName" jsonschema:"required"`
		Columns   []Column `json:"columns" jsonschema:"required"`
		Indexes   []Index  `json:"indexes,omitempty" jsonschema:"optional"`
	}

	Column struct {
		Name         string `json:"name" jsonschema:"required"`
		DataType     string `json:"dataType" jsonschema:"required"`
		IsPrimary    bool   `json:"isPrimary" jsonschema:"required"`
		IsUnique     bool   `json:"isUnique" jsonschema:"required"`
		IsNullable   bool   `json:"isNullable" jsonschema:"required"`
		DefaultValue string `json:"defaultValue,omitempty" jsonschema:"optional"`
	}

	IndexType string

	Index struct {
		Name string    `json:"name" jsonschema:"required"`
		Type IndexType `json:"type" jsonschema:"required"`
	}

	HNSWIndexOption struct {
		DistanceMetric string `json:"distanceMetric" jsonschema:"required"`
		M              int    `json:"m" jsonschema:"required"`
		EfConstruction int    `json:"efConstruction" jsonschema:"required"`
	}

	IVFFlatIndexOption struct {
		DistanceMetric string `json:"distanceMetric" jsonschema:"required"`
		Nlist          int    `json:"nlist" jsonschema:"required"`
	}
)

func (t *TableSchema) SchemaType() string {
	return "pgvector"
}
