package retryer

import (
	"github.com/megaease/easegateway/pkg/context"
	"github.com/megaease/easegateway/pkg/object/httppipeline"
	"github.com/megaease/easegateway/pkg/supervisor"
)

const (
	// Kind is the kind of Retryer.
	Kind = "Retryer"
)

var (
	results = []string{}
)

func init() {
	httppipeline.Register(&Retryer{})
}

type (
	Policy struct {
		Name               string
		MaxAttempts        int
		WaitDuration       int
		BackOffPolicy      string
		RandomWaitInterval float64
	}

	URLRule struct {
		// URLRule
		policy *Policy
	}

	Spec struct {
		Policies         []*Policy
		DefaultPolicyRef string
		URLs             []URLRule
	}

	Retryer struct {
		super    *supervisor.Supervisor
		pipeSpec *httppipeline.FilterSpec
		spec     *Spec
	}
)

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

func (r *Retryer) reload() {
}

// Init initializes Retryer.
func (r *Retryer) Init(pipeSpec *httppipeline.FilterSpec, super *supervisor.Supervisor) {
	r.pipeSpec = pipeSpec
	r.spec = pipeSpec.FilterSpec().(*Spec)
	r.super = super
	r.reload()
}

// Inherit inherits previous generation of Retryer.
func (r *Retryer) Inherit(pipeSpec *httppipeline.FilterSpec, previousGeneration httppipeline.Filter, super *supervisor.Supervisor) {
	previousGeneration.Close()
	r.Init(pipeSpec, super)
}

// Handle handles HTTP request
func (r *Retryer) Handle(ctx context.HTTPContext) string {
	return ""
}

// Status returns Status genreated by Runtime.
func (r *Retryer) Status() interface{} {
	return nil
}

// Close closes Retryer.
func (r *Retryer) Close() {
}
