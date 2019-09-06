package httpproxy

import (
	"fmt"
	"sync"

	"github.com/megaease/easegateway/pkg/context"
	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/object/httppipeline"
	"github.com/megaease/easegateway/pkg/object/httpserver"
	"github.com/megaease/easegateway/pkg/plugin/backend"
	"github.com/megaease/easegateway/pkg/plugin/fallback"
	"github.com/megaease/easegateway/pkg/plugin/ratelimiter"
	"github.com/megaease/easegateway/pkg/plugin/requestadaptor"
	"github.com/megaease/easegateway/pkg/plugin/responseadaptor"
	"github.com/megaease/easegateway/pkg/plugin/validator"
	"github.com/megaease/easegateway/pkg/scheduler"

	yaml "gopkg.in/yaml.v2"
)

const (
	// Kind is HTTPProxy kind.
	Kind = "HTTPProxy"
)

func init() {
	scheduler.Register(&scheduler.ObjectRecord{
		Kind:              Kind,
		DefaultSpecFunc:   DefaultSpec,
		NewFunc:           New,
		DependObjectKinds: []string{httpserver.Kind, httppipeline.Kind},
	})
}

type (
	// HTTPProxy is Object HTTPProxy.
	HTTPProxy struct {
		spec *Spec

		pipeline *httppipeline.HTTPPipeline
	}

	// Spec describes the HTTPProxy.
	Spec struct {
		V string `yaml:"-" v:"parent"`

		scheduler.ObjectMeta `yaml:",inline"`

		Validator       *validator.Spec       `yaml:"validator,omitempty"`
		Fallback        *fallback.Spec        `yaml:"fallback,omitempty"`
		RateLimiter     *ratelimiter.Spec     `yaml:"rateLimiter,omitempty"`
		RequestAdaptor  *requestadaptor.Spec  `yaml:"requestAdaptor,omitempty"`
		Backend         *backend.Spec         `yaml:"backend" v:"required"`
		ResponseAdaptor *responseadaptor.Spec `yaml:"responseAdaptor"`
	}

	// Status is the wrapper of httppipeline.Status.
	Status = httppipeline.Status
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
		ObjectMeta: scheduler.ObjectMeta{
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
func New(spec *Spec, prev *HTTPProxy, handlers *sync.Map) *HTTPProxy {
	var prevPipeline *httppipeline.HTTPPipeline
	if prev != nil {
		prevPipeline = prev.pipeline
	}
	hp := &HTTPProxy{
		pipeline: httppipeline.New(spec.toHTTPPipelineSpec(), prevPipeline, handlers),
	}

	// NOTE: It's expected to cover what httppipeline.New stored into handlers.
	handlers.Store(spec.Name, hp)

	return hp
}

// DefaultSpec returns HTTPProxy default spec.
func DefaultSpec() *Spec {
	return &Spec{}
}

// Handle handles all incoming traffic.
func (hp *HTTPProxy) Handle(ctx context.HTTPContext) {
	hp.pipeline.Handle(ctx)
}

// Status returns Status genreated by Runtime.
// NOTE: Caller must not call Status while reloading.
func (hp *HTTPProxy) Status() *Status {
	return hp.pipeline.Status()
}

// Close closes HTTPProxy.
func (hp *HTTPProxy) Close() {
	hp.pipeline.Close()
}
