package ratelimiter

import (
	"fmt"
	"net/http"
	"reflect"
	"time"

	"github.com/megaease/easegateway/pkg/context"
	"github.com/megaease/easegateway/pkg/filter/resilience"
	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/object/httppipeline"
	"github.com/megaease/easegateway/pkg/supervisor"
	librl "github.com/megaease/easegateway/pkg/util/ratelimiter"
)

const (
	// Kind is the kind of RateLimiter.
	Kind = "RateLimiter"
)

var (
	results = []string{}
)

func init() {
	httppipeline.Register(&RateLimiter{})
}

type (
	// Policy defines the policy of a rate limiter
	Policy struct {
		Name               string `yaml:"name" jsonschema:"required"`
		TimeoutDuration    string `yaml:"timeoutDuration" jsonschema:"omitempty,format=duration"`
		LimitRefreshPeriod string `yaml:"limitRefreshPeriod" jsonschema:"omitempty,format=duration"`
		LimitForPeriod     int    `yaml:"limitForPeriod" jsonschema:"omitempty,minimum=1"`
	}

	// RateLimiterURLRule defines the rate limiter rule for a URL pattern
	URLRule struct {
		resilience.URLRule `yaml:",inline"`
		policy             *Policy
		rl                 *librl.RateLimiter
	}

	// Spec is the configuration of a rate limiter
	Spec struct {
		Policies         []*Policy  `yaml:"policies" jsonschema:"required"`
		DefaultPolicyRef string     `yaml:"defaultPolicyRef" jsonschema:"omitempty"`
		URLs             []*URLRule `yaml:"urls" jsonschema:"required"`
	}

	// RateLimiter defines the rate limiter
	RateLimiter struct {
		super    *supervisor.Supervisor
		pipeSpec *httppipeline.FilterSpec
		spec     *Spec
	}
)

// Validate implements custom validation for Spec
func (spec Spec) Validate() error {
URLLoop:
	for _, u := range spec.URLs {
		name := u.PolicyRef
		if name == "" {
			name = spec.DefaultPolicyRef
		}

		for _, p := range spec.Policies {
			if p.Name == name {
				continue URLLoop
			}
		}

		return fmt.Errorf("policy '%s' is not defined", name)
	}

	return nil
}

func (url *URLRule) createRateLimiter() {
	policy := librl.Policy{
		LimitForPeriod: url.policy.LimitForPeriod,
	}

	if policy.LimitForPeriod == 0 {
		policy.LimitForPeriod = 50
	}

	if d := url.policy.TimeoutDuration; d != "" {
		policy.TimeoutDuration, _ = time.ParseDuration(d)
	} else {
		policy.TimeoutDuration = 100 * time.Millisecond
	}

	if d := url.policy.LimitRefreshPeriod; d != "" {
		policy.LimitRefreshPeriod, _ = time.ParseDuration(d)
	} else {
		policy.LimitRefreshPeriod = 10 * time.Millisecond
	}

	url.rl = librl.New(&policy)
}

// Kind returns the kind of RateLimiter.
func (rl *RateLimiter) Kind() string {
	return Kind
}

// DefaultSpec returns the default spec of RateLimiter.
func (rl *RateLimiter) DefaultSpec() interface{} {
	return &Spec{}
}

// Description returns the description of RateLimiter
func (rl *RateLimiter) Description() string {
	return "RateLimiter implements a rate limiter for http request."
}

// Results returns the results of RateLimiter.
func (rl *RateLimiter) Results() []string {
	return results
}

func (rl *RateLimiter) setStateListenerForURL(u *URLRule) {
	u.rl.SetStateListener(func(event *librl.Event) {
		logger.Infof("state of rate limiter '%s' on URL(%s) transited to %s at %d",
			rl.pipeSpec.Name(),
			u.ID(),
			event.State,
			event.Time.UnixNano()/1e6,
		)
	})
}

func (rl *RateLimiter) createRateLimiterForURL(u *URLRule) {
	u.Init()

	name := u.PolicyRef
	if name == "" {
		name = rl.spec.DefaultPolicyRef
	}

	for _, p := range rl.spec.Policies {
		if p.Name == name {
			u.policy = p
			break
		}
	}

	u.createRateLimiter()
	rl.setStateListenerForURL(u)
}

func isSamePolicy(spec1, spec2 *Spec, policyName string) bool {
	if policyName == "" {
		if spec1.DefaultPolicyRef != spec2.DefaultPolicyRef {
			return false
		}
		policyName = spec1.DefaultPolicyRef
	}

	var p1, p2 *Policy
	for _, p := range spec1.Policies {
		if p.Name == policyName {
			p1 = p
			break
		}
	}

	for _, p := range spec2.Policies {
		if p.Name == policyName {
			p2 = p
			break
		}
	}

	return reflect.DeepEqual(p1, p2)
}

func (rl *RateLimiter) reload(previousGeneration *RateLimiter) {
	if previousGeneration == nil {
		for _, u := range rl.spec.URLs {
			rl.createRateLimiterForURL(u)
		}
		return
	}

OuterLoop:
	for _, url := range rl.spec.URLs {
		for _, prev := range previousGeneration.spec.URLs {
			if !url.DeepEqual(&prev.URLRule) {
				continue
			}
			if !isSamePolicy(rl.spec, previousGeneration.spec, url.PolicyRef) {
				continue
			}

			url.Init()
			url.rl = prev.rl
			prev.rl = nil
			rl.setStateListenerForURL(url)
			continue OuterLoop
		}
		rl.createRateLimiterForURL(url)
	}
}

// Init initializes RateLimiter.
func (rl *RateLimiter) Init(pipeSpec *httppipeline.FilterSpec, super *supervisor.Supervisor) {
	rl.pipeSpec = pipeSpec
	rl.spec = pipeSpec.FilterSpec().(*Spec)
	rl.super = super
	rl.reload(nil)
}

// Inherit inherits previous generation of RateLimiter.
func (rl *RateLimiter) Inherit(pipeSpec *httppipeline.FilterSpec, previousGeneration httppipeline.Filter, super *supervisor.Supervisor) {
	rl.pipeSpec = pipeSpec
	rl.spec = pipeSpec.FilterSpec().(*Spec)
	rl.super = super
	rl.reload(previousGeneration.(*RateLimiter))
}

// Handle handles HTTP request
func (rl *RateLimiter) Handle(ctx context.HTTPContext) string {
	for _, u := range rl.spec.URLs {
		if !u.Match(ctx.Request()) {
			continue
		}
		if u.rl.WaitPermission() {
			return ""
		}
		ctx.Response().SetStatusCode(http.StatusTooManyRequests)
		return "rateLimiter"
	}
	return ""
}

// Status returns Status genreated by Runtime.
func (rl *RateLimiter) Status() interface{} {
	return nil
}

// Close closes RateLimiter.
func (rl *RateLimiter) Close() {
}
