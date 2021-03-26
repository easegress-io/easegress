package validator

import (
	"net/http"

	"github.com/megaease/easegateway/pkg/context"
	"github.com/megaease/easegateway/pkg/object/httppipeline"
	"github.com/megaease/easegateway/pkg/supervisor"
	"github.com/megaease/easegateway/pkg/util/httpheader"
	"github.com/megaease/easegateway/pkg/util/signer"
	"github.com/megaease/easegateway/pkg/util/stringtool"
)

const (
	// Kind is the kind of Validator.
	Kind = "Validator"

	resultInvalid = "invalid"
)

var (
	results = []string{resultInvalid}
)

func init() {
	httppipeline.Register(&Validator{})
}

type (
	// Validator is filter Validator.
	Validator struct {
		super    *supervisor.Supervisor
		pipeSpec *httppipeline.FilterSpec
		spec     *Spec

		headers *httpheader.Validator
		jwt     *JWTValidator
		signer  *signer.Signer
	}

	// Spec describes the Validator.
	Spec struct {
		Headers   *httpheader.ValidatorSpec `yaml:"headers,omitempty" jsonschema:"omitempty"`
		JWT       *JWTValidatorSpec         `yaml:"jwt,omitempty" jsonschema:"omitempty"`
		Signature *signer.Spec              `yaml:"signature,omitempty" jsonschema:"omitempty"`
	}
)

// Kind returns the kind of Validator.
func (v *Validator) Kind() string {
	return Kind
}

// DefaultSpec returns default spec of Validator.
func (v *Validator) DefaultSpec() interface{} {
	return &Spec{}
}

// Description returns the description of Validator.
func (v *Validator) Description() string {
	return "Validator validates http request."
}

// Results returns the results of Validator.
func (v *Validator) Results() []string {
	return results
}

// Init initializes Validator.
func (v *Validator) Init(pipeSpec *httppipeline.FilterSpec, super *supervisor.Supervisor) {
	v.pipeSpec, v.spec, v.super = pipeSpec, pipeSpec.FilterSpec().(*Spec), super
	v.reload()
}

// Inherit inherits previous generation of Validator.
func (v *Validator) Inherit(pipeSpec *httppipeline.FilterSpec,
	previousGeneration httppipeline.Filter, super *supervisor.Supervisor) {

	previousGeneration.Close()
	v.Init(pipeSpec, super)
}

func (v *Validator) reload() {
	if v.spec.Headers != nil {
		v.headers = httpheader.NewValidator(v.spec.Headers)
	}

	if v.spec.JWT != nil {
		v.jwt = NewJWTValidator(v.spec.JWT)
	}

	if v.spec.Signature != nil {
		v.signer = signer.CreateFromSpec(v.spec.Signature)
	}
}

// Handle validates HTTPContext.
func (v *Validator) Handle(ctx context.HTTPContext) string {
	result := v.handle(ctx)
	return ctx.CallNextHandler(result)
}

func (v *Validator) handle(ctx context.HTTPContext) string {
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
