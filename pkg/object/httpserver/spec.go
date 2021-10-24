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

package httpserver

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"regexp"

	"github.com/megaease/easegress/pkg/tracing"
	"github.com/megaease/easegress/pkg/util/ipfilter"
)

type (
	// Spec describes the HTTPServer.
	Spec struct {
		HTTP3            bool          `yaml:"http3" jsonschema:"omitempty"`
		Port             uint16        `yaml:"port" jsonschema:"required,minimum=1"`
		KeepAlive        bool          `yaml:"keepAlive" jsonschema:"required"`
		KeepAliveTimeout string        `yaml:"keepAliveTimeout" jsonschema:"omitempty,format=duration"`
		MaxConnections   uint32        `yaml:"maxConnections" jsonschema:"omitempty,minimum=1"`
		HTTPS            bool          `yaml:"https" jsonschema:"required"`
		CacheSize        uint32        `yaml:"cacheSize" jsonschema:"omitempty"`
		XForwardedFor    bool          `yaml:"xForwardedFor" jsonschema:"omitempty"`
		Tracing          *tracing.Spec `yaml:"tracing" jsonschema:"omitempty"`
		CaCertBase64     string        `yaml:"caCertBase64" jsonschema:"omitempty,format=base64"`

		// Support multiple certs, preserve the certbase64 and keybase64
		// for backward compatibility
		CertBase64 string `yaml:"certBase64" jsonschema:"omitempty,format=base64"`
		KeyBase64  string `yaml:"keyBase64" jsonschema:"omitempty,format=base64"`

		// Certs saved as map, key is domain name, value is cert
		Certs map[string]string `yaml:"certs" jsonschema:"omitempty"`
		// Keys saved as map, key is domain name, value is secret
		Keys map[string]string `yaml:"keys" jsonschema:"omitempty"`

		IPFilter *ipfilter.Spec `yaml:"ipFilter,omitempty" jsonschema:"omitempty"`
		Rules    []*Rule        `yaml:"rules" jsonschema:"omitempty"`
	}

	// Rule is first level entry of router.
	Rule struct {
		// NOTICE: If the field is a pointer, it must have `omitempty` in tag `yaml`
		// when it has `omitempty` in tag `jsonschema`.
		// Otherwise it will output null value, which is invalid in json schema (the type is object).
		// the original reason is the jsonscheme(genjs) has not support multiple types.
		// Reference: https://github.com/alecthomas/jsonschema/issues/30
		// In the future if we have the scenario where we need marshal the field, but omitempty
		// in the schema, we are suppose to support multiple types on our own.
		IPFilter   *ipfilter.Spec `yaml:"ipFilter,omitempty" jsonschema:"omitempty"`
		Host       string         `yaml:"host" jsonschema:"omitempty"`
		HostRegexp string         `yaml:"hostRegexp" jsonschema:"omitempty,format=regexp"`
		Paths      []*Path        `yaml:"paths" jsonschema:"omitempty"`
	}

	// Path is second level entry of router.
	Path struct {
		IPFilter      *ipfilter.Spec `yaml:"ipFilter,omitempty" jsonschema:"omitempty"`
		Path          string         `yaml:"path,omitempty" jsonschema:"omitempty,pattern=^/"`
		PathPrefix    string         `yaml:"pathPrefix,omitempty" jsonschema:"omitempty,pattern=^/"`
		PathRegexp    string         `yaml:"pathRegexp,omitempty" jsonschema:"omitempty,format=regexp"`
		RewriteTarget string         `yaml:"rewriteTarget" jsonschema:"omitempty"`
		Methods       []string       `yaml:"methods,omitempty" jsonschema:"omitempty,uniqueItems=true,format=httpmethod-array"`
		Backend       string         `yaml:"backend" jsonschema:"required"`
		Headers       []*Header      `yaml:"headers" jsonschema:"omitempty"`
	}

	// Header is the third level entry of router. A header entry is always under a specific path entry, that is to mean
	// the headers entry will only be checked after a path entry matched. However, the headers entry has a higher priority
	// than the path entry itself.
	Header struct {
		Key    string   `yaml:"key" jsonschema:"required"`
		Values []string `yaml:"values,omitempty" jsonschema:"omitempty,uniqueItems=true"`
		Regexp string   `yaml:"regexp,omitempty" jsonschema:"omitempty,format=regexp"`

		headerRE *regexp.Regexp
	}
)

// Validate validates HTTPServerSpec.
func (spec *Spec) Validate() error {
	if spec.HTTP3 && !spec.HTTPS {
		return fmt.Errorf("https is disabled when http3 enabled")
	}

	if spec.HTTPS {
		if spec.CertBase64 == "" && spec.KeyBase64 == "" && len(spec.Certs) == 0 && len(spec.Keys) == 0 {
			return fmt.Errorf("certBase64/keyBase64, certs/keys are both empty when https enabled")
		}
		_, err := spec.tlsConfig()
		if err != nil {
			return err
		}
	}

	return nil
}

func (spec *Spec) tlsConfig() (*tls.Config, error) {
	var certificates []tls.Certificate
	if spec.CertBase64 != "" && spec.KeyBase64 != "" {
		// Prefer add CertBase64 and KeyBase64
		certPem, _ := base64.StdEncoding.DecodeString(spec.CertBase64)
		keyPem, _ := base64.StdEncoding.DecodeString(spec.KeyBase64)
		cert, err := tls.X509KeyPair(certPem, keyPem)
		if err != nil {
			return nil, fmt.Errorf("generate x509 key pair failed: %v", err)
		}
		certificates = append(certificates, cert)
	}

	for k, v := range spec.Certs {
		if secret, exists := spec.Keys[k]; exists {
			cert, err := tls.X509KeyPair([]byte(v), []byte(secret))
			if err != nil {
				return nil, fmt.Errorf("generate x509 key pair for %s failed: %s ", k, err)
			}
			certificates = append(certificates, cert)
		} else {
			return nil, fmt.Errorf("certs %s hasn't secret corresponded to it", k)
		}
	}

	if len(certificates) == 0 {
		return nil, fmt.Errorf("none valid certs and secret")
	}

	tlsConf := &tls.Config{
		Certificates: certificates,
	}

	// if caCertBase64 configuration is provided, should enable tls.ClientAuth and
	// add the root cert
	if len(spec.CaCertBase64) != 0 {
		rootCertPem, _ := base64.StdEncoding.DecodeString(spec.CaCertBase64)
		certPool := x509.NewCertPool()
		certPool.AppendCertsFromPEM(rootCertPem)

		tlsConf.ClientAuth = tls.RequireAndVerifyClientCert
		tlsConf.ClientCAs = certPool
	}

	return tlsConf, nil
}

func (h *Header) initHeaderRoute() {
	h.headerRE = regexp.MustCompile(h.Regexp)
}

// Validate validates Header.
func (h *Header) Validate() error {
	if len(h.Values) == 0 && h.Regexp == "" {
		return fmt.Errorf("both of values and regexp are empty for key: %s", h.Key)
	}

	return nil
}
