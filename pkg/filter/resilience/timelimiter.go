package resilience

import "github.com/megaease/easegateway/pkg/context"

type (
	TimeLimiterURLRule struct {
		URLRule
		TimeoutDuration int
	}

	TimeLimiterSpec struct {
		DefaultTimeoutDuration int
		URLs                   []TimeLimiterURLRule
	}

	TimeLimiter struct {
		spec *TimeLimiterSpec
	}
)

func NewTimeLimiter(spec *TimeLimiterSpec) *TimeLimiter {
	return nil
}

func (tl *TimeLimiter) Handle(ctx context.HTTPContext) string {
	return ""
}
