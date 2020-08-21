package base

const (
	// CancelTagKey is the key of tag with random value.
	// Its appearance means the span should be dropped.
	// NOTE: Every tracing implementor must support the feature.
	CancelTagKey = "__EaseGateway_Tracing_Cancel"
)
