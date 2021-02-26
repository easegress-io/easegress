package urlratelimiter

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"github.com/megaease/easegateway/pkg/context"
	"github.com/megaease/easegateway/pkg/filter/ratelimiter"
	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/object/httppipeline"
	"github.com/megaease/easegateway/pkg/supervisor"
	"github.com/megaease/easegateway/pkg/util/fallback"
	"github.com/megaease/easegateway/pkg/util/httpheader"
)

const (
	// Kind is the kind of URLRateLimiter.
	Kind = "URLRateLimiter"

	resultTimeout  = "timeout"
	resultFallback = "fallback"
)

var (
	results = []string{resultTimeout, resultFallback}
)

func init() {
	httppipeline.Register(&URLRateLimiter{})
}

type (
	// URLRateLimiter is the entity to complete rate limiting grouped by url.
	URLRateLimiter struct {
		super    *supervisor.Supervisor
		pipeSpec *httppipeline.FilterSpec
		spec     *Spec

		paths []*path
	}

	// Spec describes URLRateLimiter.
	Spec struct {
		Fallback *fallback.Spec `yaml:"fallback,omitempty" jsonschema:"omitempty"`
		Paths    []*pathSpec    `yaml:"paths" jsonschema:"required,minItems=1"`
	}

	// Status contains status info of URLRateLimiter.
	Status struct {
		Paths []*ratelimiter.Status `yaml:"paths"`
	}

	path struct {
		spec *pathSpec

		rl *ratelimiter.RateLimiter
	}

	pathSpec struct {
		Path       string                    `yaml:"path,omitempty" jsonschema:"omitempty,pattern=^/"`
		PathPrefix string                    `yaml:"pathPrefix,omitempty" jsonschema:"omitempty,pattern=^/"`
		PathRegexp string                    `yaml:"pathRegexp,omitempty" jsonschema:"omitempty,format=regexp"`
		Headers    *httpheader.ValidatorSpec `yaml:"headers,omitempty" jsonschema:"omitempty"`

		TPS     uint32 `yaml:"tps" jsonschema:"required,minimum=1"`
		Timeout string `yaml:"timeout" jsonschema:"omitempty,format=duration"`

		pathRE           *regexp.Regexp
		headersValidator *httpheader.Validator
	}
)

// Validate validates Spec.
func (s Spec) Validate() error {
	for i := 0; i < len(s.Paths)-1; i++ {
		for j := i + 1; j < len(s.Paths); j++ {
			x, y := s.Paths[i], s.Paths[j]
			if x.sameTraffic(y) {
				return fmt.Errorf("same traffic in different paths[%d] and paths[%d]",
					i, j)
			}
		}
	}

	return nil
}

// Validate validates pathSpec.
func (s pathSpec) Validate() error {
	if s.Path == "" && s.PathPrefix == "" &&
		s.PathRegexp == "" && s.Headers == nil {
		return fmt.Errorf("none of traffic filter(path,pathPrefix,pathRegexp,headers) specified")
	}

	return nil
}

func (s *pathSpec) sameTraffic(other *pathSpec) bool {
	trafficSpec := func(spec *pathSpec) *pathSpec {
		return &pathSpec{
			Path:       spec.Path,
			PathPrefix: spec.PathPrefix,
			PathRegexp: spec.PathRegexp,
			Headers:    spec.Headers,
		}
	}

	x := trafficSpec(s)
	y := trafficSpec(other)

	return reflect.DeepEqual(x, y)
}

func (s *pathSpec) init() {
	if s.PathRegexp != "" {
		var err error
		s.pathRE, err = regexp.Compile(s.PathRegexp)
		// defensive programming
		if err != nil {
			logger.Errorf("BUG: compile %s failed: %v",
				s.PathRegexp, err)
		}
	}

	if s.Headers != nil {
		s.headersValidator = httpheader.NewValidator(s.Headers)
	}
}

func (s *pathSpec) toRateLimiterPipeSpec(fallbackSpec *fallback.Spec) *httppipeline.FilterSpec {
	meta := &httppipeline.FilterMetaSpec{
		Kind: ratelimiter.Kind,
		Name: "urlratelimiter",
	}
	filterSpec := &ratelimiter.Spec{
		TPS:      s.TPS,
		Timeout:  s.Timeout,
		Fallback: fallbackSpec,
	}

	pipeSpec, err := httppipeline.NewFilterSpec(meta, filterSpec)
	if err != nil {
		panic(err)
	}

	return pipeSpec
}

func newPath(spec *pathSpec, fallbackSpec *fallback.Spec,
	prev *path, super *supervisor.Supervisor) *path {

	spec.init()

	var prevRateLimiter *ratelimiter.RateLimiter
	if prev != nil {
		prevRateLimiter = prev.rl
	}

	rl := &ratelimiter.RateLimiter{}
	if prevRateLimiter == nil {
		rl.Init(spec.toRateLimiterPipeSpec(fallbackSpec), super)
	} else {
		rl.Inherit(spec.toRateLimiterPipeSpec(fallbackSpec), prevRateLimiter, super)
	}

	p := &path{
		spec: spec,
		rl:   rl,
	}

	return p
}

func (p *path) sameTraffic(other *pathSpec) bool {
	return p.spec.sameTraffic(other)
}

func (p *path) match(ctx context.HTTPContext) bool {
	s := p.spec
	r := ctx.Request()

	if s.Path != "" && s.Path == r.Path() {
		return true
	}
	if s.PathPrefix != "" && strings.HasPrefix(r.Path(), s.PathPrefix) {
		return true
	}
	if s.pathRE != nil {
		return s.pathRE.MatchString(r.Path())
	}
	if s.headersValidator != nil && s.headersValidator.Validate(r.Header()) == nil {
		return true
	}

	return false
}

func (p *path) handle(ctx context.HTTPContext) string {
	return p.rl.Handle(ctx)
}

func (p *path) status() interface{} {
	return p.rl.Status()
}

func (p *path) close() {
	p.rl.Close()
}

// Kind returns the kind of URLRateLimiter.
func (url *URLRateLimiter) Kind() string {
	return Kind
}

// DefaultSpec returns default spec of URLRateLimiter.
func (url *URLRateLimiter) DefaultSpec() interface{} {
	return &Spec{}
}

// Description returns the description of URLRateLimiter.
func (url *URLRateLimiter) Description() string {
	return "URLRateLimiter do the rate limiting for different URLs."
}

// Results returns the results of URLRateLimiter.
func (url *URLRateLimiter) Results() []string {
	return results
}

// Init initializes URLRateLimiter.
func (url *URLRateLimiter) Init(pipeSpec *httppipeline.FilterSpec, super *supervisor.Supervisor) {
	url.pipeSpec, url.spec, url.super = pipeSpec, pipeSpec.FilterSpec().(*Spec), super
	url.reload(nil /*no previous generation*/)
}

// Inherit inherits previous generation of URLRateLimiter.
func (url *URLRateLimiter) Inherit(pipeSpec *httppipeline.FilterSpec,
	previousGeneration httppipeline.Filter, super *supervisor.Supervisor) {

	url.pipeSpec, url.spec, url.super = pipeSpec, pipeSpec.FilterSpec().(*Spec), super
	url.reload(previousGeneration.(*URLRateLimiter))

	// NOTE: Inherited already, can't close here.
	// previousGeneration.Close()
}

func (url *URLRateLimiter) reload(previousGeneration *URLRateLimiter) {
	if previousGeneration == nil {
		for _, pathSpec := range url.spec.Paths {
			url.paths = append(url.paths, newPath(pathSpec, url.spec.Fallback, nil, url.super))
		}

		return
	}

	for _, pathSpec := range url.spec.Paths {
		var prevPath *path
		for i, path := range previousGeneration.paths {
			if path != nil && path.sameTraffic(pathSpec) {
				prevPath = path
				previousGeneration.paths[i] = nil
				break
			}
		}

		url.paths = append(url.paths, newPath(pathSpec, url.spec.Fallback, prevPath, url.super))
	}

	for _, path := range previousGeneration.paths {
		if path != nil {
			path.close()
		}
	}
}

// Handle limits HTTPContext.
func (url *URLRateLimiter) Handle(ctx context.HTTPContext) string {
	for _, path := range url.paths {
		if path.match(ctx) {
			return path.handle(ctx)
		}
	}

	return ""
}

// Status returns URLRateLimiter status.
func (url *URLRateLimiter) Status() interface{} {
	s := &Status{}

	for _, path := range url.paths {
		s.Paths = append(s.Paths, path.status().(*ratelimiter.Status))
	}

	return s
}

// Close closes URLRateLimiter.
func (url *URLRateLimiter) Close() {
	for _, path := range url.paths {
		path.close()
	}
}
