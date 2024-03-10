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

package signer

import "time"

// Spec defines the configuration of a Signer
type Spec struct {
	Literal         *Literal          `json:"literal,omitempty"`
	HeaderHoisting  *HeaderHoisting   `json:"headerHoisting,omitempty"`
	IgnoredHeaders  []string          `json:"ignoredHeaders,omitempty" jsonschema:"uniqueItems=true"`
	ExcludeBody     bool              `json:"excludeBody,omitempty"`
	TTL             string            `json:"ttl,omitempty" jsonschema:"format=duration"`
	AccessKeyID     string            `json:"accessKeyId,omitempty"`
	AccessKeySecret string            `json:"accessKeySecret,omitempty"`
	AccessKeys      map[string]string `json:"accessKeys,omitempty"`
	// TODO: AccessKeys is used as an internal access key store, but an external store is also needed
}

type idSecretMap map[string]string

func (m idSecretMap) GetSecret(id string) (string, bool) {
	s, ok := m[id]
	return s, ok
}

// CreateFromSpec create a Signer from configuration
func CreateFromSpec(spec *Spec) *Signer {
	signer := New()

	signer.SetCredential(spec.AccessKeyID, spec.AccessKeySecret)

	if spec.Literal != nil {
		signer.SetLiteral(spec.Literal)
	}

	if spec.HeaderHoisting != nil {
		signer.SetHeaderHoisting(spec.HeaderHoisting)
	}

	signer.IgnoreHeader(spec.IgnoredHeaders...)
	signer.ExcludeBody(spec.ExcludeBody)

	if ttl, e := time.ParseDuration(spec.TTL); e == nil {
		signer.SetTTL(ttl)
	}

	if len(spec.AccessKeys) > 0 {
		signer.SetAccessKeyStore(idSecretMap(spec.AccessKeys))
	}
	return signer
}
