package httpproxy

import (
	"fmt"
	"sync"

	"github.com/megaease/easegateway/pkg/context"
	"github.com/megaease/easegateway/pkg/filter/backend"
	"github.com/megaease/easegateway/pkg/filter/corsadaptor"
	"github.com/megaease/easegateway/pkg/filter/fallback"
	"github.com/megaease/easegateway/pkg/filter/ratelimiter"
	"github.com/megaease/easegateway/pkg/filter/requestadaptor"
	"github.com/megaease/easegateway/pkg/filter/responseadaptor"
	"github.com/megaease/easegateway/pkg/filter/urlratelimiter"
	"github.com/megaease/easegateway/pkg/filter/validator"
	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/object/httppipeline"
	"github.com/megaease/easegateway/pkg/object/httpserver"
	"github.com/megaease/easegateway/pkg/supervisor"

	yaml "gopkg.in/yaml.v2"
)

const (
	// Kind is HTTPProxy kind.
	Kind = "HTTPProxy"
)

func init() {
	supervisor.Register(&supervisor.ObjectRecord{
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
		supervisor.ObjectMeta `yaml:",inline"`

		Validator       *validator.Spec       `yaml:"validator,omitempty" jsonschema:"omitempty"`
		Fallback        *fallback.Spec        `yaml:"fallback,omitempty" jsonschema:"omitempty"`
		CORSAdaptor     *corsadaptor.Spec     `yaml:"corsAdaptor,omitempty" jsonschema:"omitempty"`
		URLRateLimiter  *urlratelimiter.Spec  `yaml:"urlRateLimiter,omitempty" jsonschema:"omitempty"`
		RateLimiter     *ratelimiter.Spec     `yaml:"rateLimiter,omitempty" jsonschema:"omitempty"`
		RequestAdaptor  *requestadaptor.Spec  `yaml:"requestAdaptor,omitempty" jsonschema:"omitempty"`
		Backend         *backend.Spec         `yaml:"backend" jsonschema:"required"`
		ResponseAdaptor *responseadaptor.Spec `yaml:"responseAdaptor,omitempty" jsonschema:"omitempty"`
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
	pipeline := spec.toHTTPPipelineSpec()
	buff, err := yaml.Marshal(pipeline)
	if err != nil {
		return fmt.Errorf("BUG: marshal %#v to yaml failed: %v", pipeline, err)
	}
	return pipeline.Validate(buff)
}

func (spec Spec) toHTTPPipelineSpec() *httppipeline.Spec {
	pipelineSpec := &httppipeline.Spec{
		ObjectMeta: supervisor.ObjectMeta{
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
		pipelineSpec.Filters = append(pipelineSpec.Filters,
			transformSpec("validator", validator.Kind, spec.Validator))
	}
	if spec.Fallback != nil {
		pipelineSpec.Filters = append(pipelineSpec.Filters,
			transformSpec("fallback", fallback.Kind, spec.Fallback))
	}
	if spec.CORSAdaptor != nil {
		pipelineSpec.Filters = append(pipelineSpec.Filters,
			transformSpec("corsAdaptor", corsadaptor.Kind, spec.CORSAdaptor))
	}
	if spec.URLRateLimiter != nil {
		pipelineSpec.Filters = append(pipelineSpec.Filters,
			transformSpec("urlRateLimiter", urlratelimiter.Kind, spec.URLRateLimiter))
	}
	if spec.RateLimiter != nil {
		pipelineSpec.Filters = append(pipelineSpec.Filters,
			transformSpec("rateLimiter", ratelimiter.Kind, spec.RateLimiter))
	}
	if spec.RequestAdaptor != nil {
		pipelineSpec.Filters = append(pipelineSpec.Filters,
			transformSpec("requestAdaptor", requestadaptor.Kind, spec.RequestAdaptor))
	}

	pipelineSpec.Filters = append(pipelineSpec.Filters,
		transformSpec("backend", backend.Kind, spec.Backend))

	if spec.ResponseAdaptor != nil {
		pipelineSpec.Filters = append(pipelineSpec.Filters,
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
	// NOTE: The HTTPPipleine.Close will Delete myself in handlers.
	hp.pipeline.Close()
}
