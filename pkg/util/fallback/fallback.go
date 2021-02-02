package fallback

import (
	"bytes"
	"strconv"

	"github.com/megaease/easegateway/pkg/util/httpheader"

	"github.com/megaease/easegateway/pkg/context"
)

type (
	// Fallback is filter Fallback.
	Fallback struct {
		spec      *Spec
		mockBody  []byte
		bodyLenth string
	}

	// Spec describes the Fallback.
	Spec struct {
		MockCode    int               `yaml:"mockCode" jsonschema:"required,format=httpcode"`
		MockHeaders map[string]string `yaml:"mockHeaders" jsonschema:"omitempty"`
		MockBody    string            `yaml:"mockBody" jsonschema:"omitempty"`
	}
)

// New creates a Fallback.
func New(spec *Spec) *Fallback {
	f := &Fallback{
		spec:     spec,
		mockBody: []byte(spec.MockBody),
	}
	f.bodyLenth = strconv.Itoa(len(f.mockBody))
	return f
}

// Fallback fallabcks HTTPContext.
func (f *Fallback) Fallback(ctx context.HTTPContext) {
	w := ctx.Response()

	w.SetStatusCode(f.spec.MockCode)
	w.Header().Set(httpheader.KeyContentLength, f.bodyLenth)
	for key, value := range f.spec.MockHeaders {
		w.Header().Set(key, value)
	}
	w.SetBody(bytes.NewReader(f.mockBody))
	ctx.AddTag("fallback")
}
