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

package certextractor

import (
	"fmt"
)

type (
	// Spec describes the CertExtractor.
	Spec struct {
		CertIndex int16  `yaml:"certIndex" jsonschema:"required"`
		Target    string `yaml:"target" "omitempty,enum=subject,enum=issuer"`
		// Different field options listed here https://pkg.go.dev/crypto/x509/pkix#Name
		Field string `yaml:"field" "omitempty,enum=Country,enum=Organization,enum=OrganizationalUnit,
													enum=Locality,enum=Province,enum=StreetAddress,enum=PostalCode
													enum=SerialNumber,enum=CommonName"`
		HeaderKey string `yaml:"headerKey" "omitempty"`
	}
)

// Validate is dummy as yaml rules already validate Spec.
func (spec *Spec) Validate() error { return nil }

// DefaultHeaderKey returns HeaderKey or the default value.
func (spec *Spec) DefaultHeaderKey() string {
	if spec.HeaderKey != "" {
		return spec.HeaderKey
	}
	return fmt.Sprintf("tls-%s-%s", spec.Target, spec.Field)
}
