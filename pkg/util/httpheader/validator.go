package httpheader

import (
	"fmt"
	"regexp"

	"github.com/megaease/easegateway/pkg/common"
	"github.com/megaease/easegateway/pkg/logger"
)

type (
	// ValidatorSpec describes Validator
	ValidatorSpec map[string]*ValueValidator

	// ValueValidator is the entity to validate value.
	ValueValidator struct {
		V string `yaml:"-" v:"parent"`

		// NOTE: It allows empty value.
		Values []string `yaml:"values" v:"unique"`
		Regexp string   `yaml:"regexp" v:"omitempty,regexp"`
		re     *regexp.Regexp
	}

	// Validator is the entity standing for all operations to HTTPHeader.
	Validator struct {
		spec *ValidatorSpec
	}
)

// Validate valites ValueValidator.
func (vv ValueValidator) Validate() error {
	if len(vv.Values) == 0 && vv.Regexp == "" {
		return fmt.Errorf("neither values nor regexp is specified")
	}

	return nil
}

// NewValidator creates a validator.
func NewValidator(spec *ValidatorSpec) *Validator {
	v := &Validator{
		spec: spec,
	}

	for _, vv := range *spec {
		if len(vv.Regexp) != 0 {
			re, err := regexp.Compile(vv.Regexp)
			if err != nil {
				logger.Errorf("BUG: compile regexp %s failed: %v",
					vv.Regexp, err)
				continue
			}
			vv.re = re
		}
	}

	return v
}

// Validate validates HTTPHeader by the Validator.
func (v Validator) Validate(h *HTTPHeader) error {
LOOP:
	for key, vv := range *v.spec {
		values := h.GetAll(key)
		for _, value := range values {
			if common.StrInSlice(value, vv.Values) {
				continue LOOP
			}
			if vv.re != nil && vv.re.MatchString(value) {
				continue LOOP
			}
			return fmt.Errorf("header %s:%s is invalid", key, value)
		}
		return fmt.Errorf("header %s not found", key)
	}

	return nil
}
