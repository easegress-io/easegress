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
	"crypto/x509/pkix"

	httpcontext "github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/object/httppipeline"
)

const (
	// Kind is the kind of CertExtractor.
	Kind = "CertExtractor"
)

var results = []string{}

func init() {
	httppipeline.Register(&CertExtractor{})
}

type (
	// CertExtractor extracts given field from TLS certificates and sets it to request headers.
	CertExtractor struct {
		filterSpec *httppipeline.FilterSpec
		spec       *Spec

		headerKey string
	}
)

// Kind returns the kind of CertExtractor.
func (ce *CertExtractor) Kind() string {
	return Kind
}

// DefaultSpec returns the default spec of CertExtractor.
func (ce *CertExtractor) DefaultSpec() interface{} {
	return &Spec{}
}

// Description returns the description of CertExtractor.
func (ce *CertExtractor) Description() string {
	return "CertExtractor extracts given field from TLS certificates and sets it to request headers."
}

// Results returns the results of CertExtractor.
func (ce *CertExtractor) Results() []string {
	return results
}

// Init initializes CertExtractor.
func (ce *CertExtractor) Init(filterSpec *httppipeline.FilterSpec) {
	ce.filterSpec, ce.spec = filterSpec, filterSpec.FilterSpec().(*Spec)

	ce.headerKey = ce.spec.DefaultHeaderKey()
}

// Inherit inherits previous generation of CertExtractor.
func (ce *CertExtractor) Inherit(filterSpec *httppipeline.FilterSpec, previousGeneration httppipeline.Filter) {
	previousGeneration.Close()
	ce.Init(filterSpec)
}

// Close closes CertExtractor.
func (ce *CertExtractor) Close() {}

// Handle retrieves header values and sets request headers.
func (ce *CertExtractor) Handle(ctx httpcontext.HTTPContext) string {
	result := ce.handle(ctx)
	return ctx.CallNextHandler(result)
}

// CertExtractor extracts given field from TLS certificates and sets it to request headers.
func (ce *CertExtractor) handle(ctx httpcontext.HTTPContext) string {
	r := ctx.Request()
	if connectionState := r.TLS(); connectionState != nil {
		certs := connectionState.PeerCertificates
		if certs != nil && len(certs) > 0 {
			n := int16(len(certs))
			// positive ce.spec.CertIndex from the beginning, negative from the end
			index := (n + ce.spec.CertIndex) % n
			cert := certs[index]

			var target pkix.Name
			if ce.spec.Target == "subject" {
				target = cert.Subject
			} else {
				target = cert.Issuer
			}

			var result []string
			switch ce.spec.Field {
			case "Country":
				result = target.Country
			case "Organization":
				result = target.Organization
			case "OrganizationalUnit":
				result = target.OrganizationalUnit
			case "Locality":
				result = target.Locality
			case "Province":
				result = target.Province
			case "StreetAddress":
				result = target.StreetAddress
			case "PostalCode":
				result = target.PostalCode
			case "SerialNumber":
				result = append(result, target.SerialNumber)
			case "CommonName":
				result = append(result, target.CommonName)
			}
			for _, res := range result {
				if res != "" {
					r.Header().Add(ce.headerKey, res)
				}
			}
		}
	}
	return ""
}

// Status returns status.
func (ce *CertExtractor) Status() interface{} { return nil }
