package httpproxy

import (
	"fmt"

	"github.com/megaease/easegateway/pkg/context"
	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/object/httppipeline"
	"github.com/megaease/easegateway/pkg/plugin/backend"
	"github.com/megaease/easegateway/pkg/plugin/fallback"
	"github.com/megaease/easegateway/pkg/plugin/ratelimiter"
	"github.com/megaease/easegateway/pkg/plugin/requestadaptor"
	"github.com/megaease/easegateway/pkg/plugin/responseadaptor"
	"github.com/megaease/easegateway/pkg/plugin/validator"
	"github.com/megaease/easegateway/pkg/registry"

	// TODO: Move the import to pkg/registry/registry.go
	_ "github.com/megaease/easegateway/pkg/plugin/remoteplugin"

	"gopkg.in/yaml.v2"
)

func init() {
	registry.Register(Kind, DefaultSpec)
}

const (
	// Kind is HTTPProxy kind.
	Kind = "HTTPProxy"
)

type (
	// HTTPProxy is Object HTTPProxy.
	HTTPProxy struct {
		spec *Spec

		pipeline *httppipeline.HTTPPipeline
	}

	// Spec describes the HTTPProxy.
	Spec struct {
		V string `yaml:"-" v:"parent"`

		registry.MetaSpec `yaml:",inline"`

		Validator       *validator.Spec       `yaml:"validator,omitempty"`
		Fallback        *fallback.Spec        `yaml:"fallback,omitempty"`
		RateLimiter     *ratelimiter.Spec     `yaml:"rateLimiter,omitempty"`
		RequestAdaptor  *requestadaptor.Spec  `yaml:"requestAdaptor,omitempty"`
		Backend         *backend.Spec         `yaml:"backend" v:"required"`
		ResponseAdaptor *responseadaptor.Spec `yaml:"responseAdaptor"`
	}
)

// Validate validates Spec.
func (spec Spec) Validate() error {
	// NOTE: The tag of v parent may be behind backend.
	if spec.Backend == nil {
		return fmt.Errorf("backend is required")
	}
	return spec.toHTTPPipelineSpec().Validate()
}

func (spec Spec) toHTTPPipelineSpec() *httppipeline.Spec {
	pipelineSpec := &httppipeline.Spec{
		MetaSpec: registry.MetaSpec{
			Name: spec.Name,
			Kind: httppipeline.Kind,
		},
	}

	transformSpec := func(name, kind string, i interface{}) map[string]interface{} {
		buff, err := yaml.Marshal(i)
		if err != nil {
			logger.Errorf("BUG: marshal %#v to yaml failed: %v", i, err)
			return nil
		}

		m := make(map[string]interface{})
		err = yaml.Unmarshal(buff, &m)
		if err != nil {
			logger.Errorf("BUG: unmarshal %s to %T failed: %v",
				buff, m, err)
			return nil
		}

		m["name"], m["kind"] = name, kind

		return m
	}

	if spec.Validator != nil {
		pipelineSpec.Plugins = append(pipelineSpec.Plugins,
			transformSpec("validator", validator.Kind, spec.Validator))
	}
	if spec.Fallback != nil {
		pipelineSpec.Plugins = append(pipelineSpec.Plugins,
			transformSpec("fallback", fallback.Kind, spec.Fallback))
	}
	if spec.RateLimiter != nil {
		pipelineSpec.Plugins = append(pipelineSpec.Plugins,
			transformSpec("rateLimiter", ratelimiter.Kind, spec.RateLimiter))
	}
	if spec.RequestAdaptor != nil {
		pipelineSpec.Plugins = append(pipelineSpec.Plugins,
			transformSpec("requestAdaptor", requestadaptor.Kind, spec.RequestAdaptor))
	}

	pipelineSpec.Plugins = append(pipelineSpec.Plugins,
		transformSpec("backend", backend.Kind, spec.Backend))

	if spec.ResponseAdaptor != nil {
		pipelineSpec.Plugins = append(pipelineSpec.Plugins,
			transformSpec("responseAdaptor", responseadaptor.Kind, spec.ResponseAdaptor))
	}

	return pipelineSpec
}

// New creates an HTTPProxy.
func New(spec *Spec, r *Runtime) *HTTPProxy {
	hp := &HTTPProxy{
		pipeline: httppipeline.New(spec.toHTTPPipelineSpec(), r.pipeline),
	}

	return hp
}

// DefaultSpec returns HTTPProxy default spec.
func DefaultSpec() registry.Spec {
	return &Spec{}
}

// Handle handles all incoming traffic.
func (hp *HTTPProxy) Handle(ctx context.HTTPContext) {
	hp.pipeline.Handle(ctx)
}

// Close closes HTTPProxy.
func (hp *HTTPProxy) Close() {
	hp.pipeline.Close()
}
