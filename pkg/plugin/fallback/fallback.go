package fallback

import (
	"bytes"

	"github.com/megaease/easegateway/pkg/context"
)

type (
	// Fallback is plugin Fallback.
	Fallback struct {
		spec     *Spec
		mockBody []byte
	}

	// Spec describes the Fallback.
	Spec struct {
		MockCode    int               `yaml:"mockCode" v:"required,httpcode"`
		MockHeaders map[string]string `yaml:"mockHeaders" v:"dive,keys,required,endkeys,required"`
		MockBody    string            `yaml:"mockBody"`
	}
)

// New creates a Fallback.
func New(spec *Spec, runtime *Runtime) *Fallback {
	return &Fallback{
		spec:     spec,
		mockBody: []byte(spec.MockBody),
	}
}

// Close closes Fallback.
// Nothing to do.
func (f *Fallback) Close() {}

// Fallback fallabcks HTTPContext.
func (f *Fallback) Fallback(ctx context.HTTPContext) {
	w := ctx.Response()

	w.SetStatusCode(f.spec.MockCode)
	for key, value := range f.spec.MockHeaders {
		w.Header().Set(key, value)
	}
	w.SetBody(bytes.NewReader(f.mockBody))
}
