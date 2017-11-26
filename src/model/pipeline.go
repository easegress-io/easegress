package model

import (
	"fmt"
	"sync"

	"github.com/hexdecteam/easegateway-types/pipelines"

	pipelines_gw "pipelines"
)

//
// Pipeline entry in model structure
//

type Pipeline struct {
	sync.RWMutex
	typ    string
	config pipelines_gw.Config
}

func newPipeline(typ string, conf pipelines_gw.Config) *Pipeline {
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

func (p *Pipeline) Config() pipelines_gw.Config {
	p.RLock()
	defer p.RUnlock()
	return p.config
}

func (p *Pipeline) GetInstance(ctx pipelines.PipelineContext, statistics *PipelineStatistics,
	m *Model, data interface{}) (pipelines_gw.Pipeline, error) {

	p.RLock()
	defer p.RUnlock()

	// Parallel pipeline could be added here in future if needed,
	// which in zhiyan's mind is DAG+deduce.

	// Copy config, e.g. plugin names, for every pipeline instances,
	// updated pipeline doesn't impact any running task
	return newLinearPipeline(ctx, statistics, p.config, m, data)
}

func (p *Pipeline) UpdateConfig(conf pipelines_gw.Config) {
	p.Lock()
	defer p.Unlock()
	p.config = conf
}

//
// Pipeline config factory
//

func GetPipelineConfig(typ string) (pipelines_gw.Config, error) {
	var conf pipelines_gw.Config
	switch typ {
	case "LinearPipeline":
		conf = linearPipelineConfigConstructor()
	default:
		return nil, fmt.Errorf("invalid pipeline type %s", typ)
	}

	return conf, nil
}
