package model

import (
	"fmt"
	"sync"

	"pipelines"
)

//
// Pipeline entry in model structure
//

type Pipeline struct {
	sync.RWMutex
	typ    string
	config pipelines.Config
}

func newPipeline(typ string, conf pipelines.Config) *Pipeline {
	return &Pipeline{
		typ:    typ,
		config: conf,
	}
}

func (p *Pipeline) Name() string {
	p.RLock()
	defer p.RUnlock()
	return p.config.PipelineName()
}

func (p *Pipeline) Type() string {
	p.RLock()
	defer p.RUnlock()
	return p.typ
}

func (p *Pipeline) Config() pipelines.Config {
	p.RLock()
	defer p.RUnlock()
	return p.config
}

func (p *Pipeline) GetInstance(ctx pipelines.PipelineContext, statistics *PipelineStatistics,
	m *Model) (pipelines.Pipeline, error) {

	p.RLock()
	defer p.RUnlock()

	// Parallel pipeline could be added here in future if needed,
	// which in zhiyan's mind is DAG+deduce.

	// Copy config, e.g. plugin names, for every pipeline instances,
	// updated pipeline doesn't impact any running task
	return newLinearPipeline(ctx, statistics, p.config, m)
}

func (p *Pipeline) UpdateConfig(conf pipelines.Config) {
	p.Lock()
	defer p.Unlock()
	p.config = conf
}

//
// Pipeline config factory
//

func GetPipelineConfig(typ string) (pipelines.Config, error) {
	var conf pipelines.Config
	switch typ {
	case "LinearPipeline":
		conf = linearPipelineConfigConstructor()
	default:
		return nil, fmt.Errorf("invalid pipeline type %s", typ)
	}

	return conf, nil
}
