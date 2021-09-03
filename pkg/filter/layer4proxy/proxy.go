package layer4proxy

import (
	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/object/layer4pipeline"
)

const (
	// Kind is the kind of Proxy.
	Kind = "Proxy"

	resultFallback      = "fallback"
	resultInternalError = "internalError"
	resultClientError   = "clientError"
	resultServerError   = "serverError"
)

var results = []string{
	resultFallback,
	resultInternalError,
	resultClientError,
	resultServerError,
}

func init() {
	layer4pipeline.Register(&Proxy{})
}

type (
	// Proxy is the filter Proxy.
	Proxy struct {
		filterSpec *layer4pipeline.FilterSpec
		spec       *Spec

		mainPool       *pool
		candidatePools []*pool
		mirrorPool     *pool
	}

	// Spec describes the Proxy.
	Spec struct {
		MainPool       *PoolSpec   `yaml:"mainPool" jsonschema:"required"`
		CandidatePools []*PoolSpec `yaml:"candidatePools,omitempty" jsonschema:"omitempty"`
		MirrorPool     *PoolSpec   `yaml:"mirrorPool,omitempty" jsonschema:"omitempty"`
	}

	// Status is the status of Proxy.
	Status struct {
		MainPool       *PoolStatus   `yaml:"mainPool"`
		CandidatePools []*PoolStatus `yaml:"candidatePools,omitempty"`
		MirrorPool     *PoolStatus   `yaml:"mirrorPool,omitempty"`
	}
)

func (p *Proxy) Kind() string {
	return Kind
}

func (p *Proxy) DefaultSpec() interface{} {
	return &Spec{}
}

func (p *Proxy) Description() string {
	return "Proxy sets the proxy of proxy servers"
}

func (p *Proxy) Results() []string {
	panic("implement me")
}

func (p *Proxy) Init(filterSpec *layer4pipeline.FilterSpec) {
	panic("implement me")
}

func (p *Proxy) Inherit(filterSpec *layer4pipeline.FilterSpec, previousGeneration layer4pipeline.Filter) {
	panic("implement me")
}

func (p *Proxy) Handle(layer4Context context.Layer4Context) (result string) {
	panic("implement me")
}

func (p *Proxy) Status() interface{} {
	panic("implement me")
}

func (p *Proxy) Close() {
	p.mainPool.close()

	if p.candidatePools != nil {
		for _, v := range p.candidatePools {
			v.close()
		}
	}

	if p.mirrorPool != nil {
		p.mirrorPool.close()
	}
}
