package validator

import (
	"github.com/megaease/easegateway/pkg/context"
	"github.com/megaease/easegateway/pkg/util/httpheader"
)

type (
	// Validator is plugin Validator.
	Validator struct {
		spec *Spec

		headers *httpheader.Validator
	}

	// Spec describes the Validator.
	Spec struct {
		Headers *httpheader.ValidatorSpec `yaml:"headers" v:"required,dive,keys,required,endkeys,required"`
	}
)

// New creates a Validator.
func New(spec *Spec, runtime *Runtime) *Validator {
	return &Validator{
		spec:    spec,
		headers: httpheader.NewValidator(spec.Headers),
	}
}

// Close closes Validator.
// Nothing to do.
func (v *Validator) Close() {}

// Validate validates HTTPContext.
func (v *Validator) Validate(ctx context.HTTPContext) error {
	return v.headers.Validate(ctx.Request().Header())
}
