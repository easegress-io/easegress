package retryer

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/megaease/easegateway/pkg/context"
	"github.com/megaease/easegateway/pkg/filter/resilience"
	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/object/httppipeline"
	"github.com/megaease/easegateway/pkg/supervisor"
	libr "github.com/megaease/easegateway/pkg/util/retryer"
)

const (
	// Kind is the kind of Retryer.
	Kind          = "Retryer"
	resultRetryer = "retryer"
)

var (
	results = []string{resultRetryer}
)

func init() {
	httppipeline.Register(&Retryer{})
}

type (
	Policy struct {
		Name                     string  `yaml:"name" jsonschema:"required"`
		MaxAttempts              int     `yaml:"maxAttempts" jsonschema:"omitempty,minimum=1"`
		WaitDuration             string  `yaml:"waitDuration" jsonschema:"omitempty,format=duration"`
		BackOffPolicy            string  `yaml:"backOffPolicy" jsonschema:"omitempty,enum=random,enum=exponential"`
		RandomizationFactor      float64 `yaml:"randomizationFactor" jsonschema:"omitempty,minimum=0,maximum=1"`
		CountingNetworkException bool    `yaml:"countingNetworkException"`
		ExceptionalStatusCode    []int   `yaml:"exceptionalStatusCode" jsonschema:"omitempty,uniqueItems=true,format=httpcode-array"`
	}

	URLRule struct {
		resilience.URLRule `yaml:",inline"`
		policy             *Policy
		r                  *libr.Retryer
	}

	Spec struct {
		Policies         []*Policy
		DefaultPolicyRef string
		URLs             []*URLRule
	}

	Retryer struct {
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

func (url *URLRule) createRetryer() {
	policy := libr.Policy{
		MaxAttempts:         url.policy.MaxAttempts,
		RandomizationFactor: url.policy.RandomizationFactor,
		BackOffPolicy:       libr.RandomBackOff,
	}

	if policy.MaxAttempts <= 0 {
		policy.MaxAttempts = 3
	}

	if policy.RandomizationFactor < 0 || policy.RandomizationFactor >= 1 {
		policy.RandomizationFactor = 0.5
	}

	if strings.ToUpper(url.policy.BackOffPolicy) == "EXPONENTIAL" {
		policy.BackOffPolicy = libr.ExponentiallyBackOff
	}

	if d := url.policy.WaitDuration; d != "" {
		policy.WaitDuration, _ = time.ParseDuration(d)
	} else {
		policy.WaitDuration = time.Millisecond * 500
	}

	url.r = libr.New(&policy)
}

// Kind returns the kind of Retryer.
func (r *Retryer) Kind() string {
	return Kind
}

// DefaultSpec returns the default spec of Retryer.
func (r *Retryer) DefaultSpec() interface{} {
	return &Spec{}
}

// Description returns the description of Retryer
func (r *Retryer) Description() string {
	return "Retryer implements a retryer for http request."
}

// Results returns the results of Retryer.
func (r *Retryer) Results() []string {
	return results
}

func (r *Retryer) setStateListenerForURL(u *URLRule) {
	u.r.SetStateListener(func(event *libr.Event) {
		logger.Infof("attempts %d of retryer on URL(%s) failed with error '%s' at %d",
			event.Attempt,
			r.pipeSpec.Name(),
			u.ID(),
			event.Error,
			event.Time.UnixNano()/1e6,
		)
	})
}

func (r *Retryer) createRetryerForURL(u *URLRule) {
	u.Init()

	name := u.PolicyRef
	if name == "" {
		name = r.spec.DefaultPolicyRef
	}

	for _, p := range r.spec.Policies {
		if p.Name == name {
			u.policy = p
			break
		}
	}

	u.createRetryer()
	r.setStateListenerForURL(u)
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

func (r *Retryer) reload(previousGeneration *Retryer) {
	if previousGeneration == nil {
		for _, url := range r.spec.URLs {
			r.createRetryerForURL(url)
		}
		return
	}

OuterLoop:
	for _, url := range r.spec.URLs {
		for _, prev := range previousGeneration.spec.URLs {
			if !url.DeepEqual(&prev.URLRule) {
				continue
			}
			if !isSamePolicy(r.spec, previousGeneration.spec, url.PolicyRef) {
				continue
			}

			url.Init()
			url.r = prev.r
			prev.r = nil
			r.setStateListenerForURL(url)
			continue OuterLoop
		}
		r.createRetryerForURL(url)
	}
}

// Init initializes Retryer.
func (r *Retryer) Init(pipeSpec *httppipeline.FilterSpec, super *supervisor.Supervisor) {
	r.pipeSpec = pipeSpec
	r.spec = pipeSpec.FilterSpec().(*Spec)
	r.super = super
	r.reload(nil)
}

// Inherit inherits previous generation of Retryer.
func (r *Retryer) Inherit(pipeSpec *httppipeline.FilterSpec, previousGeneration httppipeline.Filter, super *supervisor.Supervisor) {
	r.pipeSpec = pipeSpec
	r.spec = pipeSpec.FilterSpec().(*Spec)
	r.super = super
	r.reload(previousGeneration.(*Retryer))
}

func (r *Retryer) handle(ctx context.HTTPContext, u *URLRule) string {
	wrapper := func(fn context.HandlerFunc) context.HandlerFunc {
		return func() string {
			var result string
			_, e := u.r.Execute(func() (interface{}, error) {
				result = fn()
				if result != "" {
					return nil, fmt.Errorf("result is: %s", result)
				}
				code := ctx.Response().StatusCode()
				for _, c := range u.policy.ExceptionalStatusCode {
					if code == c {
						return nil, fmt.Errorf("status code is: %d", code)
					}
				}

				return nil, nil
			})
			if e != nil {
				return resultRetryer
			}
			return result
		}
	}

	ctx.AddHandlerWrapper("retryer", wrapper)
	return ""
}

// Handle handles HTTP request
func (r *Retryer) Handle(ctx context.HTTPContext) string {
	for _, u := range r.spec.URLs {
		if u.Match(ctx.Request()) {
			return r.handle(ctx, u)
		}
	}
	return ""
}

// Status returns Status genreated by Runtime.
func (r *Retryer) Status() interface{} {
	return nil
}

// Close closes Retryer.
func (r *Retryer) Close() {
}
