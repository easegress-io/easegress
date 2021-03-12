package circuitbreaker

import (
	"fmt"
	"regexp"
	"time"

	"github.com/megaease/easegateway/pkg/context"
	"github.com/megaease/easegateway/pkg/util/ratelimiter"
)

type (
	// RateLimiterPolicy defines the policy of a rate limiter
	RateLimiterPolicy struct {
		Name               string `yaml:"name" jsonschema:"required"`
		TimeoutDuration    string `yaml:"timeoutDuration" jsonschema:"omitempty,format=duration"`
		LimitRefreshPeriod string `yaml:"limitRefreshPeriod" jsonschema:"omitempty,format=duration"`
		LimitForPeriod     int64  `yaml:"limitForPeriod" jsonschema:"omitempty,minimum=1"`
	}

	// RateLimiterURLRule defines the rate limiter rule for a URL pattern
	RateLimiterURLRule struct {
		URLRule `yaml:",inline"`
		policy  *RateLimiterPolicy
		limiter *ratelimiter.RateLimiter
	}

	// RateLimiterSpec is the configuration of a rate limiter
	RateLimiterSpec struct {
		Policies         []*RateLimiterPolicy  `yaml:"policies" jsonschema:"required"`
		DefaultPolicyRef string                `yaml:"defaultPolicyRef" jsonschema:"omitempty"`
		URLs             []*RateLimiterURLRule `yaml:"urls" jsonschema:"required"`
	}

	// RateLimiter defines the rate limiter
	RateLimiter struct {
		spec *RateLimiterSpec
	}
)

// Validate implements custom validation for RateLimiterSpec
func (spec RateLimiterSpec) Validate() error {
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

func (url *RateLimiterURLRule) createRateLimiter() {
	policy := ratelimiter.Policy{
		LimitForPeriod: url.policy.LimitForPeriod,
	}

	if policy.LimitForPeriod == 0 {
		policy.LimitForPeriod = 50
	}

	if d := url.policy.TimeoutDuration; d != "" {
		policy.TimeoutDuration, _ = time.ParseDuration(d)
	} else {
		policy.TimeoutDuration = 5 * time.Second
	}

	if d := url.policy.LimitRefreshPeriod; d != "" {
		policy.LimitRefreshPeriod, _ = time.ParseDuration(d)
	} else {
		policy.LimitRefreshPeriod = 500 * time.Nanosecond
	}

	url.limiter = ratelimiter.New(&policy)
}

// NewRateLimiter creates a new rate limiter from spec
func NewRateLimiter(spec *RateLimiterSpec) *RateLimiter {
	cb := &RateLimiter{spec: spec}

	for _, u := range spec.URLs {
		if u.URL.RegEx != "" {
			u.URL.re = regexp.MustCompile(u.URL.RegEx)
		}

		name := u.PolicyRef
		if name == "" {
			name = spec.DefaultPolicyRef
		}

		for _, p := range spec.Policies {
			if p.Name == name {
				u.policy = p
				break
			}
		}

		u.createRateLimiter()
	}

	return cb
}

// Handle handles HTTP request
func (rl *RateLimiter) Handle(ctx context.HTTPContext) string {
	for _, u := range rl.spec.URLs {
		if !u.Match(ctx.Request()) {
			continue
		}
		if u.limiter.AccquirePermission() {
			return ""
		}
		return "rateLimiter"
	}
	return ""
}
