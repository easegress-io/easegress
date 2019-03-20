package registry

// MetaSpec is the fundamental specification for all objects
// which want to register at registry.
type MetaSpec struct {
	Name string `yaml:"name" v:"required,urlname"`
	Kind string `yaml:"kind" v:"required"`

	// Placeholder
	// Labels will be exported by supporting deploying to corresponding nodes.
	// Labels map[string]string
}

// GetName gets name.
func (ms *MetaSpec) GetName() string { return ms.Name }

// GetKind gets kind.
func (ms *MetaSpec) GetKind() string { return ms.Kind }
