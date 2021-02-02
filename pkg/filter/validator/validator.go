package validator

import (
	"net/http"

	"github.com/megaease/easegateway/pkg/context"
	"github.com/megaease/easegateway/pkg/object/httppipeline"
	"github.com/megaease/easegateway/pkg/util/httpheader"
	"github.com/megaease/easegateway/pkg/util/signer"
	"github.com/megaease/easegateway/pkg/util/stringtool"
)

const (
	// Kind is the kind of Validator.
	Kind = "Validator"

	resultInvalid = "invalid"
)

func init() {
	httppipeline.Register(&httppipeline.FilterRecord{
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
	// Validator is filter Validator.
	Validator struct {
		spec *Spec

		headers *httpheader.Validator
		jwt     *JWTValidator
		signer  *signer.Signer
	}

	// Spec describes the Validator.
	Spec struct {
		httppipeline.FilterMeta `yaml:",inline"`

		Headers   *httpheader.ValidatorSpec `yaml:"headers,omitempty" jsonschema:"omitempty"`
		JWT       *JWTValidatorSpec         `yaml:"jwt,omitempty" jsonschema:"omitempty"`
		Signature *signer.Spec              `yaml:"signature,omitempty" jsonschema:"omitempty"`
	}
)

// New creates a Validator.
func New(spec *Spec, prev *Validator) *Validator {
	v := &Validator{spec: spec}

	if spec.Headers != nil {
		v.headers = httpheader.NewValidator(spec.Headers)
	}

	if spec.JWT != nil {
		v.jwt = NewJWTValidator(spec.JWT)
	}

	if spec.Signature != nil {
		v.signer = signer.CreateFromSpec(spec.Signature)
	}

	return v
}

// Handle validates HTTPContext.
func (v *Validator) Handle(ctx context.HTTPContext) string {
	req := ctx.Request()

	if v.headers != nil {
		err := v.headers.Validate(req.Header())
		if err != nil {
			ctx.Response().SetStatusCode(http.StatusBadRequest)
			ctx.AddTag(stringtool.Cat("header validator: ", err.Error()))
			return resultInvalid
		}
	}

	if v.jwt != nil {
		err := v.jwt.Validate(req)
		if err != nil {
			ctx.Response().SetStatusCode(http.StatusForbidden)
			ctx.AddTag(stringtool.Cat("JWT validator: ", err.Error()))
			return resultInvalid
		}
	}

	if v.signer != nil {
		err := v.signer.Verify(req.Std())
		if err != nil {
			ctx.Response().SetStatusCode(http.StatusForbidden)
			ctx.AddTag(stringtool.Cat("signature validator: ", err.Error()))
			return resultInvalid
		}
	}

	return ""
}

// Status returns status.
func (v *Validator) Status() interface{} { return nil }

// Close closes Validator.
func (v *Validator) Close() {}
