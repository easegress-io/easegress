package resilience

import "github.com/megaease/easegateway/pkg/context"

type (
	RateLimiterPolicy struct {
		Name               string
		TimeoutDuration    int64
		LimitRefreshPeriod uint64
		LimitForPeriod     int32
	}

	RateLimiterURLRule struct {
		URLRule
		policy *RateLimiterPolicy
	}

	RateLimiterSpec struct {
		Policies         []RateLimiterPolicy
		DefaultPolicyRef string
		defaultPolicy    *RateLimiterPolicy
		URLs             []RateLimiterURLRule
	}

	RateLimiter struct {
		spec *RateLimiterSpec
	}
)

func NewRateLimiter(spec *RateLimiterSpec) *RateLimiter {
	return nil
}

func (rl *RateLimiter) Handle(ctx context.HTTPContext) string {
	return ""
}
