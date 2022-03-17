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

package fallback

import (
	"bytes"
	"strconv"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/util/httpheader"
)

type (
	// Fallback is filter Fallback.
	Fallback struct {
		spec       *Spec
		mockBody   []byte
		bodyLength string
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
	f.bodyLength = strconv.Itoa(len(f.mockBody))
	return f
}

// Fallback fallbacks HTTPContext.
func (f *Fallback) Fallback(ctx context.Context) {
	w := ctx.Response()

	w.SetStatusCode(f.spec.MockCode)
	w.Header().Set(httpheader.KeyContentLength, f.bodyLength)
	for key, value := range f.spec.MockHeaders {
		w.Header().Set(key, value)
	}
	w.SetBody(bytes.NewReader(f.mockBody))
	ctx.AddTag("fallback")
}
