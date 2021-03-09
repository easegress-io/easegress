package resilience

import "github.com/megaease/easegateway/pkg/context"

type (
	RetryerPolicy struct {
		Name               string
		MaxAttempts        int
		WaitDuration       int
		BackOffPolicy      string
		RandomWaitInterval float64
	}

	RetryerURLRule struct {
		URLRule
		policy *RetryerPolicy
	}

	RetryerSpec struct {
		Policies         []RetryerPolicy
		DefaultPolicyRef string
		defaultPolicy    *RetryerPolicy
		URLs             []RetryerURLRule
	}

	Retryer struct {
		spec *RetryerSpec
	}
)

func NewRetryer(spec *RetryerSpec) *Retryer {
	return nil
}

func (r *Retryer) Handle(ctx context.HTTPContext) string {
	return ""
}
