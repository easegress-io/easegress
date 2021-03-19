package retryer

import (
	"bytes"
	"fmt"
	"io/ioutil"
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
		Policies         []*Policy  `yaml:"policies" jsonschema:"required"`
		DefaultPolicyRef string     `yaml:"defaultPolicyRef" jsonschema:"omitempty"`
		URLs             []*URLRule `yaml:"urls" jsonschema:"required"`
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

// Init initializes Retryer.
func (r *Retryer) Init(pipeSpec *httppipeline.FilterSpec, super *supervisor.Supervisor) {
	r.pipeSpec = pipeSpec
	r.spec = pipeSpec.FilterSpec().(*Spec)
	r.super = super
	for _, url := range r.spec.URLs {
		r.createRetryerForURL(url)
	}
}

// Inherit inherits previous generation of Retryer.
func (r *Retryer) Inherit(pipeSpec *httppipeline.FilterSpec, previousGeneration httppipeline.Filter, super *supervisor.Supervisor) {
	r.Init(pipeSpec, super)
}

func (r *Retryer) handle(ctx context.HTTPContext, u *URLRule) string {
	wrapper := func(fn context.HandlerFunc) context.HandlerFunc {
		return func() string {
			var result string
			data, _ := ioutil.ReadAll(ctx.Request().Body())

			_, e := u.r.Execute(func() (interface{}, error) {
				ctx.Request().SetBody(bytes.NewReader(data))
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
