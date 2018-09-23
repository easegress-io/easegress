package config

type Store interface {
	GetAllPlugins() ([]*PluginSpec, error)
	GetAllPipelines() ([]*PipelineSpec, error)
	AddPlugin(plugin *PluginSpec) error
	AddPipeline(pipeline *PipelineSpec) error
	DeletePlugin(name string) error
	DeletePipeline(name string) error
	UpdatePlugin(plugin *PluginSpec) error
	UpdatePipeline(pipeline *PipelineSpec) error
}

func InitStore() (Store, error) {
	store, err := NewJSONFileStore()
	if err != nil {
		return nil, err
	}

	return store, nil
}
