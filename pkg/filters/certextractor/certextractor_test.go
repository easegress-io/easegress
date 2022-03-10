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
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/megaease/easegress/pkg/context/contexttest"
	"github.com/megaease/easegress/pkg/filters"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/supervisor"
	"github.com/megaease/easegress/pkg/util/httpheader"
	"github.com/megaease/easegress/pkg/util/yamltool"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	logger.InitNop()
	code := m.Run()
	os.Exit(code)
}

func createCertExtractor(
	yamlSpec string, prev *CertExtractor, supervisor *supervisor.Supervisor) (*CertExtractor, error) {
	rawSpec := make(map[string]interface{})
	yamltool.Unmarshal([]byte(yamlSpec), &rawSpec)
	spec, err := filters.NewSpec(supervisor, "", rawSpec)
	if err != nil {
		return nil, err
	}
	hl := &CertExtractor{}
	if prev == nil {
		hl.Init(spec)
	} else {
		hl.Inherit(spec, prev)
	}
	return hl, nil
}

const yaml = `
kind: "CertExtractor"
name: "cn-extractor"
certIndex: -1
target: "subject"
field: "CommonName"
headerKey: "key"
`

func TestSpec(t *testing.T) {
	assert := assert.New(t)

	const yamlNoKey = `
kind: "CertExtractor"
name: "cn-extractor"
certIndex: -1
target: "subject"
field: "CommonName"
`
	ce, err := createCertExtractor(yaml, nil, nil)
	assert.Nil(err)
	assert.Nil(ce.Status())
	ce, err = createCertExtractor(yaml, ce, nil)
	assert.Nil(err)
	assert.Equal(ce.headerKey, "key")

	ce, err = createCertExtractor(yamlNoKey, ce, nil)
	assert.Nil(err)
	assert.Equal(ce.headerKey, "tls-subject-CommonName")
}

func prepareCtxAndHeader(connState *tls.ConnectionState) (*contexttest.MockedHTTPContext, http.Header) {
	ctx := &contexttest.MockedHTTPContext{}
	header := http.Header{}
	stdr := &http.Request{}
	stdr.TLS = connState
	ctx.MockedRequest.MockedStd = func() *http.Request {
		return stdr
	}
	ctx.MockedRequest.MockedHeader = func() *httpheader.HTTPHeader {
		return httpheader.New(header)
	}
	return ctx, header
}

func TestHandle(t *testing.T) {
	assert := assert.New(t)

	t.Run("no TLS", func(t *testing.T) {
		ctx := &contexttest.MockedHTTPContext{}
		ce, _ := createCertExtractor(yaml, nil, nil)
		assert.Equal("", ce.Handle(ctx))

		peerCertificates := make([]*x509.Certificate, 0)
		connState := &tls.ConnectionState{PeerCertificates: peerCertificates}
		ctx, _ = prepareCtxAndHeader(connState)
		ce, _ = createCertExtractor(yaml, nil, nil)
		assert.Equal("", ce.Handle(ctx))
	})

	t.Run("TLS has no field", func(t *testing.T) {
		peerCertificates := make([]*x509.Certificate, 0)
		peerCertificates = append(peerCertificates, &x509.Certificate{
			Subject: pkix.Name{},
			Issuer:  pkix.Name{},
		})
		connState := &tls.ConnectionState{PeerCertificates: peerCertificates}
		ctx, _ := prepareCtxAndHeader(connState)
		ce, _ := createCertExtractor(yaml, nil, nil)
		assert.Equal("", ce.Handle(ctx))
	})

	t.Run("TLS has field", func(t *testing.T) {
		peerCertificates := make([]*x509.Certificate, 0)
		peerCertificates = append(peerCertificates, &x509.Certificate{
			Subject: pkix.Name{
				Country:            []string{"1"},
				Organization:       []string{"2"},
				OrganizationalUnit: []string{"3"},
				Locality:           []string{"4"},
				Province:           []string{"5"},
				StreetAddress:      []string{"6"},
				PostalCode:         []string{"7"},
				SerialNumber:       "8",
				CommonName:         "INFO-1",
			},
			Issuer: pkix.Name{
				CommonName: "INFO-2",
			},
		})
		connState := &tls.ConnectionState{PeerCertificates: peerCertificates}
		t.Run("subject", func(t *testing.T) {
			ctx, header := prepareCtxAndHeader(connState)
			subjectYaml := yaml
			ce, _ := createCertExtractor(subjectYaml, nil, nil)
			assert.Equal("", ce.handle(ctx))
			assert.Equal("INFO-1", header.Get("key"))
		})

		t.Run("issuer", func(t *testing.T) {
			ctx, header := prepareCtxAndHeader(connState)
			issuerYaml := strings.ReplaceAll(yaml, `target: "subject"`, `target: "issuer"`)
			ce, _ := createCertExtractor(issuerYaml, nil, nil)
			assert.Equal("", ce.handle(ctx))
			assert.Equal("INFO-2", header.Get("key"))
		})

		t.Run("test all fields", func(t *testing.T) {
			ctx, header := prepareCtxAndHeader(connState)
			fields := []string{"Country", "Organization", "OrganizationalUnit", "Locality",
				"Province", "StreetAddress", "PostalCode", "SerialNumber",
			}
			for _, fieldName := range fields {
				ce, _ := createCertExtractor(
					strings.ReplaceAll(yaml, `field: "CommonName"`, fmt.Sprintf(`field: "%s"`, fieldName)),
					nil,
					nil,
				)
				assert.Equal("", ce.Handle(ctx))
			}
			assert.Equal([]string{"1", "2", "3", "4", "5", "6", "7", "8"}, header["Key"])
		})
		t.Run("multiple certs", func(t *testing.T) {
			for _, val := range []string{"second", "third", "fourth"} {
				peerCertificates = append(peerCertificates, &x509.Certificate{
					Subject: pkix.Name{
						Province: []string{val},
					},
				})

			}
			connState := &tls.ConnectionState{PeerCertificates: peerCertificates}
			ctx, header := prepareCtxAndHeader(connState)
			yamlConfig := `
kind: "CertExtractor"
name: "cn-extractor"
certIndex: -2 # second last certificate
target: "subject"
field: "Province"
`
			ce, _ := createCertExtractor(yamlConfig, nil, nil)
			assert.Equal("", ce.Handle(ctx))
			assert.Equal("third", header.Get("tls-subject-province"))

			ctx, header = prepareCtxAndHeader(connState)
			yamlConfig2 := strings.ReplaceAll(yamlConfig, "certIndex: -2", "certIndex: -15")
			ce, _ = createCertExtractor(yamlConfig2, nil, nil)
			assert.Equal("", ce.Handle(ctx))
			assert.Equal("second", header.Get("tls-subject-province"))
		})
	})
}
