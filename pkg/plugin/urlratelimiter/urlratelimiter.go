package urlratelimiter

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"github.com/megaease/easegateway/pkg/context"
	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/object/httppipeline"
	"github.com/megaease/easegateway/pkg/plugin/ratelimiter"
	"github.com/megaease/easegateway/pkg/util/fallback"
	"github.com/megaease/easegateway/pkg/util/httpheader"
)

const (
	// Kind is the kind of URLRateLimiter.
	Kind = "URLRateLimiter"

	resultTimeout  = "timeout"
	resultFallback = "fallback"
)

func init() {
	httppipeline.Register(&httppipeline.PluginRecord{
		Kind:            Kind,
		DefaultSpecFunc: DefaultSpec,
		NewFunc:         New,
		Results:         []string{resultTimeout, resultFallback},
	})
}

// DefaultSpec returns default spec.
func DefaultSpec() *Spec {
	return &Spec{}
}

type (
	// URLRateLimiter is the entity to complete rate limiting grouped by url.
	URLRateLimiter struct {
		spec *Spec

		paths []*path
	}

	// Spec describes URLRateLimiter.
	Spec struct {
		V string `yaml:"-" v:"parent"`

		httppipeline.PluginMeta `yaml:",inline"`

		Fallback *fallback.Spec `yaml:"fallback"`
		Paths    []*pathSpec    `yaml:"paths" v:"required,dive"`
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
		V string `yaml:"-" v:"parent"`

		Path       string                    `yaml:"path,omitempty" v:"omitempty,prefix=/"`
		PathPrefix string                    `yaml:"pathPrefix,omitempty" v:"omitempty,prefix=/"`
		PathRegexp string                    `yaml:"pathRegexp,omitempty" v:"omitempty,regexp"`
		Headers    *httpheader.ValidatorSpec `yaml:"headers,omitempty" v:"omitempty,dive,keys,required,endkeys,required"`

		TPS     uint32 `yaml:"tps" v:"gte=1"`
		Timeout string `yaml:"timeout" v:"omitempty,duration,dmin=1ms"`

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

func (s *pathSpec) toRateLimiterSpec(fallbackSpec *fallback.Spec) *ratelimiter.Spec {
	return &ratelimiter.Spec{
		PluginMeta: httppipeline.PluginMeta{
			Kind: ratelimiter.Kind,
			Name: "urlratelimiter",
		},
		TPS:      s.TPS,
		Timeout:  s.Timeout,
		Fallback: fallbackSpec,
	}
}

func newPath(spec *pathSpec, fallbackSpec *fallback.Spec, prev *path) *path {
	spec.init()

	var prevRL *ratelimiter.RateLimiter
	if prev != nil {
		prevRL = prev.rl
	}

	p := &path{
		spec: spec,
		rl:   ratelimiter.New(spec.toRateLimiterSpec(fallbackSpec), prevRL),
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

func (p *path) status() *ratelimiter.Status {
	return p.rl.Status()
}

func (p *path) close() {
	p.rl.Close()
}

// New creates a URLRateLimiter.
func New(spec *Spec, prev *URLRateLimiter) *URLRateLimiter {
	url := &URLRateLimiter{spec: spec}

	if prev == nil {
		for _, pathSpec := range spec.Paths {
			url.paths = append(url.paths, newPath(pathSpec, spec.Fallback, nil))
		}

		return url
	}

	for _, pathSpec := range spec.Paths {
		var prevPath *path
		for i, path := range prev.paths {
			if path != nil && path.sameTraffic(pathSpec) {
				prevPath = path
				prev.paths[i] = nil
				break
			}
		}

		url.paths = append(url.paths, newPath(pathSpec, spec.Fallback, prevPath))
	}

	for _, path := range prev.paths {
		if path != nil {
			path.close()
		}
	}

	return url
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
func (url *URLRateLimiter) Status() *Status {
	s := &Status{}
	for _, path := range url.paths {
		s.Paths = append(s.Paths, path.status())
	}

	return s
}

// Close closes URLRateLimiter.
func (url *URLRateLimiter) Close() {
	for _, path := range url.paths {
		path.close()
	}
}
