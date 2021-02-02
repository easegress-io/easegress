package fallback

import (
	"github.com/megaease/easegateway/pkg/context"
	"github.com/megaease/easegateway/pkg/object/httppipeline"
	"github.com/megaease/easegateway/pkg/util/fallback"
)

const (
	// Kind is the kind of Fallback.
	Kind = "Fallback"

	resultFallback = "fallback"
)

func init() {
	httppipeline.Register(&httppipeline.FilterRecord{
		Kind:            Kind,
		DefaultSpecFunc: DefaultSpec,
		NewFunc:         New,
		Results:         []string{resultFallback},
	})
}

// DefaultSpec returns default spec.
func DefaultSpec() *Spec {
	return &Spec{}
}

type (
	// Fallback is filter Fallback.
	Fallback struct {
		f *fallback.Fallback
	}

	// Spec describes the Fallback.
	Spec struct {
		httppipeline.FilterMeta `yaml:",inline"`
		fallback.Spec           `yaml:",inline"`
	}
)

// New creates a Fallback.
func New(spec *Spec, prev *Fallback) *Fallback {
	return &Fallback{
		f: fallback.New(&spec.Spec),
	}
}

// Handle fallabcks HTTPContext.
// It always returns fallback.
func (f *Fallback) Handle(ctx context.HTTPContext) string {
	f.f.Fallback(ctx)
	return resultFallback
}

// Status returns Status.
func (f *Fallback) Status() interface{} { return nil }

// Close closes Fallback.
func (f *Fallback) Close() {}
