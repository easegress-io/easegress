package corsadaptor

import (
	"net/http"

	"github.com/megaease/easegateway/pkg/context"
	"github.com/megaease/easegateway/pkg/object/httppipeline"

	"github.com/rs/cors"
)

const (
	// Kind is kind of CORSAdaptor.
	Kind = "CORSAdaptor"

	resultFallback = "fallback"
)

func init() {
	httppipeline.Register(&httppipeline.PluginRecord{
		Kind:            Kind,
		DefaultSpecFunc: DefaultSpec,
		NewFunc:         New,
		Results:         []string{resultFallback},
	})
}

// DefaultSpec return default spes.
func DefaultSpec() *Spec {
	return &Spec{}
}

type (
	// CORSAdaptor is plugin for CORS request.
	CORSAdaptor struct {
		spec *Spec

		cors *cors.Cors
	}

	// Spec is describes of CORSAdaptor.
	Spec struct {
		httppipeline.PluginMeta `yaml:",inline"`

		AllowedOrigins   []string `yaml:"allowedOrigins" jsonschema:"omitempty"`
		AllowedMethods   []string `yaml:"allowedMethods" jsonschema:"omitempty,uniqueItems=true,format=httpmethod-array"`
		AllowedHeaders   []string `yaml:"allowedHeaders" jsonschema:"omitempty"`
		AllowCredentials bool     `yaml:"allowCredentials" jsonschema:"omitempty"`
		ExposedHeaders   []string `yaml:"exposedHeaders" jsonschema:"omitempty"`
	}
)

// New for create a CORSAdaptor.
func New(spec *Spec, prev *CORSAdaptor) *CORSAdaptor {
	return &CORSAdaptor{
		spec: spec,
		cors: cors.New(cors.Options{
			AllowedOrigins:   spec.AllowedOrigins,
			AllowedMethods:   spec.AllowedMethods,
			AllowedHeaders:   spec.AllowedHeaders,
			AllowCredentials: spec.AllowCredentials,
			ExposedHeaders:   spec.ExposedHeaders,
		}),
	}
}

// Handle for handles simple cross-origin requests or directs.
func (f *CORSAdaptor) Handle(ctx context.HTTPContext) string {
	r := ctx.Request()
	w := ctx.Response()
	method := r.Method()
	headerAllowMethod := r.Header().Get("Access-Control-Request-Method")
	if method == http.MethodOptions && headerAllowMethod != "" {
		f.cors.HandlerFunc(w.Std(), r.Std())
		return resultFallback
	}
	return ""
}

// Status return status.
func (f *CORSAdaptor) Status() interface{} { return nil }

// Close close CORSAdaptor.
func (f *CORSAdaptor) Close() {}
