package validator

import (
	"net/http"

	"github.com/megaease/easegateway/pkg/context"
	"github.com/megaease/easegateway/pkg/object/httppipeline"
	"github.com/megaease/easegateway/pkg/util/httpheader"
	"github.com/megaease/easegateway/pkg/util/stringtool"
)

const (
	// Kind is the kind of Validator.
	Kind = "Validator"

	resultInvalid = "invalid"
)

func init() {
	httppipeline.Register(&httppipeline.PluginRecord{
		Kind:            Kind,
		DefaultSpecFunc: DefaultSpec,
		NewFunc:         New,
		Results:         []string{resultInvalid},
	})
}

// DefaultSpec returns default spec.
func DefaultSpec() *Spec {
	return &Spec{}
}

type (
	// Validator is plugin Validator.
	Validator struct {
		spec *Spec

		headers *httpheader.Validator
	}

	// Spec describes the Validator.
	Spec struct {
		httppipeline.PluginMeta `yaml:",inline"`

		Headers *httpheader.ValidatorSpec `yaml:"headers" jsonschema:"required"`
	}
)

// New creates a Validator.
func New(spec *Spec, prev *Validator) *Validator {
	return &Validator{
		spec:    spec,
		headers: httpheader.NewValidator(spec.Headers),
	}
}

// Handle validates HTTPContext.
func (v *Validator) Handle(ctx context.HTTPContext) string {
	err := v.headers.Validate(ctx.Request().Header())
	if err != nil {
		ctx.Response().SetStatusCode(http.StatusBadRequest)
		ctx.AddTag(stringtool.Cat("validator: ", err.Error()))
		return resultInvalid
	}
	return ""
}

// Status returns status.
func (v *Validator) Status() interface{} { return nil }

// Close closes Validator.
func (v *Validator) Close() {}
