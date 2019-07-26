package httpfilter

import (
	"fmt"
	"hash/fnv"
	"math/rand"
	"regexp"
	"time"

	"github.com/megaease/easegateway/pkg/common"
	"github.com/megaease/easegateway/pkg/context"
	"github.com/megaease/easegateway/pkg/logger"
)

func init() {
	rand.Seed(time.Now().Unix())
}

const (
	policyIPHash     string = "ipHash"
	policyHeaderHash        = "headerHash"
	policyRandom            = "random"
)

type (
	// Spec describes HTTPFilter.
	Spec struct {
		V string `yaml:"-" v:"parent"`

		Headers     map[string]*ValueFilter `yaml:"headers" v:"dive,keys,required,endkeys,required"`
		Probability *Probability            `yaml:"probability" v:"omitempty"`
	}

	// ValueFilter describes value.
	ValueFilter struct {
		V string `yaml:"-" v:"parent"`

		// NOTE: It allows empty value.
		Values []string `yaml:"values" v:"unique"`
		Regexp string   `yaml:"regexp" v:"omitempty,regexp"`
		re     *regexp.Regexp
	}

	// HTTPFilter filters HTTP traffic.
	HTTPFilter struct {
		spec *Spec
	}

	// Probability filters HTTP traffic by probability.
	Probability struct {
		V string `yaml:"-" v:"parent"`

		PerMill       uint32 `yaml:"perMill" v:"required,gte=1,lte=1000"`
		Policy        string `yaml:"policy" v:"required,oneof=ipHash headerHash random"`
		HeaderHashKey string `yaml:"headerHashKey"`
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

// Validate validates ValueFilter.
func (vf ValueFilter) Validate() error {
	if len(vf.Values) == 0 && vf.Regexp == "" {
		return fmt.Errorf("neither values nor regexp is specified")
	}

	return nil
}

// New creates an HTTPFilter.
func New(spec *Spec) *HTTPFilter {
	hf := &HTTPFilter{
		spec: spec,
	}

	for _, vf := range spec.Headers {
		if len(vf.Regexp) != 0 {
			re, err := regexp.Compile(vf.Regexp)
			if err != nil {
				logger.Errorf("BUG: compile regexp %s failed: %v",
					vf.Regexp, err)
				continue
			}
			vf.re = re
		}
	}

	return hf
}

// Filter filters HTTPContext.
func (hf *HTTPFilter) Filter(ctx context.HTTPContext) bool {
	if len(hf.spec.Headers) > 0 {
		return hf.filterHeader(ctx)
	}

	return hf.filterProbability(ctx)
}

func (hf *HTTPFilter) filterHeader(ctx context.HTTPContext) bool {
	h := ctx.Request().Header()
	for key, vf := range hf.spec.Headers {
		values := h.GetAll(key)
		for _, value := range values {
			if common.StrInSlice(value, vf.Values) {
				return true
			}
			if vf.re != nil && vf.re.MatchString(value) {
				return true
			}
		}

	}

	return false
}

func (hf *HTTPFilter) hash32Once(key string) uint32 {
	hash := fnv.New32a()
	hash.Write([]byte(key))
	return hash.Sum32()
}

func (hf *HTTPFilter) filterProbability(ctx context.HTTPContext) bool {
	prob := hf.spec.Probability

	var result uint32
	switch prob.Policy {
	case policyIPHash:
		result = hf.hash32Once(ctx.Request().RealIP())
	case policyHeaderHash:
		result = hf.hash32Once(ctx.Request().Header().Get(prob.HeaderHashKey))
	case policyRandom:
		result = uint32(rand.Int31n(1000))
	default:
		logger.Errorf("BUG: unsupported probability policy: %s", prob.Policy)
		result = hf.hash32Once(ctx.Request().RealIP())
	}

	if result%1000 < prob.PerMill {
		return true
	}
	return false
}
