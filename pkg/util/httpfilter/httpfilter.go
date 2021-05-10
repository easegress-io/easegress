package httpfilter

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/megaease/easegateway/pkg/context"
	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/util/hashtool"
	"github.com/megaease/easegateway/pkg/util/urlrule"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

const (
	policyIPHash     string = "ipHash"
	policyHeaderHash        = "headerHash"
	policyRandom            = "random"
)

type (
	// Spec describes HTTPFilter.
	Spec struct {
		Headers     map[string]*urlrule.StringMatch `yaml:"headers" jsonschema:"omitempty"`
		URLs        []*urlrule.URLRule              `yaml:"urls" jsonschema:"omitempty"`
		Probability *Probability                    `yaml:"probability,omitempty" jsonschema:"omitempty"`
	}

	// HTTPFilter filters HTTP traffic.
	HTTPFilter struct {
		spec *Spec
	}

	// Probability filters HTTP traffic by probability.
	Probability struct {
		PerMill       uint32 `yaml:"perMill" jsonschema:"required,minimum=1,maximum=1000"`
		Policy        string `yaml:"policy" jsonschema:"required,enum=ipHash,enum=headerHash,enum=random"`
		HeaderHashKey string `yaml:"headerHashKey" jsonschema:"omitempty"`
	}
)

// Validate validates Probability.
func (p Probability) Validate() error {
	if p.Policy == policyHeaderHash && p.HeaderHashKey == "" {
		return fmt.Errorf("headerHash needs to speficy headerHashKey")
	}

	return nil
}

// Validate validates Spec
func (s Spec) Validate() error {
	if len(s.Headers) == 0 && s.Probability == nil {
		return fmt.Errorf("none of headers and probability is specified")
	}

	if len(s.Headers) > 0 && s.Probability != nil {
		return fmt.Errorf("both headers and probability are specified")
	}

	return nil
}

// New creates an HTTPFilter.
func New(spec *Spec) *HTTPFilter {
	hf := &HTTPFilter{
		spec: spec,
	}

	for _, stringMatcher := range spec.Headers {
		stringMatcher.Init()
	}

	for _, url := range spec.URLs {
		url.Init()
	}

	return hf
}

// Filter filters HTTPContext.
func (hf *HTTPFilter) Filter(ctx context.HTTPContext) bool {
	if len(hf.spec.Headers) > 0 {
		matchHeader := hf.filterHeader(ctx)
		if len(hf.spec.URLs) > 0 {
			return matchHeader && hf.filterURL(ctx)
		}

		return matchHeader
	}

	return hf.filterProbability(ctx)
}

func (hf *HTTPFilter) filterHeader(ctx context.HTTPContext) bool {
	h := ctx.Request().Header()
	headerMatch := false
	for key, vf := range hf.spec.Headers {
		if headerMatch {
			break
		}

		values := h.GetAll(key)

		for _, value := range values {
			if vf.Match(value) {
				headerMatch = true
				break
			}
		}
	}

	return headerMatch
}

func (hf *HTTPFilter) filterURL(ctx context.HTTPContext) bool {
	req := ctx.Request()
	urlMatch := false
	for _, url := range hf.spec.URLs {
		if url.Match(req) {
			urlMatch = true
			break
		}
	}
	return urlMatch
}

func (hf *HTTPFilter) filterProbability(ctx context.HTTPContext) bool {
	prob := hf.spec.Probability

	var result uint32
	switch prob.Policy {
	case policyIPHash:
		result = hashtool.Hash32(ctx.Request().RealIP())
	case policyHeaderHash:
		result = hashtool.Hash32(ctx.Request().Header().Get(prob.HeaderHashKey))
	case policyRandom:
		result = uint32(rand.Int31n(1000))
	default:
		logger.Errorf("BUG: unsupported probability policy: %s", prob.Policy)
		result = hashtool.Hash32(ctx.Request().RealIP())
	}

	if result%1000 < prob.PerMill {
		return true
	}
	return false
}
