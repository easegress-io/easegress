/*
 * Copyright (c) 2017, The Easegress Authors
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

// Package pathadaptor provides a path adaptor.
package pathadaptor

import (
	"regexp"
	"strings"

	"github.com/megaease/easegress/v2/pkg/logger"
)

type (
	// Spec describes rules for PathAdaptor.
	Spec struct {
		Replace       string         `json:"replace,omitempty"`
		AddPrefix     string         `json:"addPrefix,omitempty" jsonschema:"pattern=^/"`
		TrimPrefix    string         `json:"trimPrefix,omitempty" jsonschema:"pattern=^/"`
		RegexpReplace *RegexpReplace `json:"regexpReplace,omitempty"`
	}

	// RegexpReplace use regexp-replace pair to rewrite path.
	RegexpReplace struct {
		Regexp  string `json:"regexp" jsonschema:"required,format=regexp"`
		Replace string `json:"replace"`

		re *regexp.Regexp
	}

	// PathAdaptor is the path Adaptor.
	PathAdaptor struct {
		spec *Spec
	}
)

// New creates a pathAdaptor.
func New(spec *Spec) *PathAdaptor {
	if spec.RegexpReplace != nil {
		var err error
		spec.RegexpReplace.re, err = regexp.Compile(spec.RegexpReplace.Regexp)
		if err != nil {
			logger.Errorf("BUG: compile regexp %s failed: %v",
				spec.RegexpReplace.Regexp, err)
		}
	}

	return &PathAdaptor{
		spec: spec,
	}
}

// Adapt adapts path.
func (pa *PathAdaptor) Adapt(path string) string {
	if len(pa.spec.Replace) != 0 {
		return pa.spec.Replace
	}

	if len(pa.spec.AddPrefix) != 0 {
		return pa.spec.AddPrefix + path
	}

	if len(pa.spec.TrimPrefix) != 0 {
		return strings.TrimPrefix(path, pa.spec.TrimPrefix)
	}

	if pa.spec.RegexpReplace != nil && pa.spec.RegexpReplace.re != nil {
		return pa.spec.RegexpReplace.re.ReplaceAllString(path,
			pa.spec.RegexpReplace.Replace)
	}

	return path
}
