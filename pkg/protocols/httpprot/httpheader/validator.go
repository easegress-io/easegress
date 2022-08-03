/*
 * Copyright (c) 2017, MegaEase
 * All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package httpheader

import (
	"fmt"
	"regexp"

	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/util/stringtool"
)

type (
	// ValidatorSpec describes Validator
	ValidatorSpec map[string]*ValueValidator

	// ValueValidator is the entity to validate value.
	ValueValidator struct {
		// NOTE: It allows empty value.
		Values []string `json:"values" jsonschema:"omitempty,uniqueItems=true"`
		Regexp string   `json:"regexp" jsonschema:"omitempty,format=regexp"`
		re     *regexp.Regexp
	}

	// Validator is the entity standing for all operations to HTTPHeader.
	Validator struct {
		spec *ValidatorSpec
	}
)

// Validate validates ValueValidator.
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
			if stringtool.StrInSlice(value, vv.Values) {
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
