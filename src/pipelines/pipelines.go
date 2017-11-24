package pipelines

type Pipeline interface {
	Name() string
	Prepare()
	Run() error
	Stop()
	Close()
}

////

type Config interface {
	PipelineName() string
	PluginNames() []string
	Parallelism() uint16
	CrossPipelineRequestBacklog() uint16
	Prepare() error
}

// Pipelines register authority

var (
	PIPELINE_TYPES = map[string]interface{}{
		"LinearPipeline": nil,
	}
)

func ValidType(t string) bool {
	_, exists := PIPELINE_TYPES[t]
	return exists
}

func GetAllTypes() []string {
	types := make([]string, 0)
	for t := range PIPELINE_TYPES {
		types = append(types, t)
	}
	return types
}
