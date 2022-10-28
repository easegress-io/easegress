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
	"net/http"
	"net/url"
	"regexp"

	"github.com/megaease/easegress/pkg/object/autocertmanager"
	"github.com/megaease/easegress/pkg/tracing"
	"github.com/megaease/easegress/pkg/util/ipfilter"
	"github.com/megaease/easegress/pkg/util/stringtool"
)

type (
	// Spec describes the HTTPServer.
	Spec struct {
		HTTP3             bool          `json:"http3" jsonschema:"omitempty"`
		KeepAlive         bool          `json:"keepAlive" jsonschema:"required"`
		HTTPS             bool          `json:"https" jsonschema:"required"`
		AutoCert          bool          `json:"autoCert" jsonschema:"omitempty"`
		XForwardedFor     bool          `json:"xForwardedFor" jsonschema:"omitempty"`
		Port              uint16        `json:"port" jsonschema:"required,minimum=1"`
		ClientMaxBodySize int64         `json:"clientMaxBodySize" jsonschema:"omitempty"`
		KeepAliveTimeout  string        `json:"keepAliveTimeout" jsonschema:"omitempty,format=duration"`
		MaxConnections    uint32        `json:"maxConnections" jsonschema:"omitempty,minimum=1"`
		CacheSize         uint32        `json:"cacheSize" jsonschema:"omitempty"`
		Tracing           *tracing.Spec `json:"tracing,omitempty" jsonschema:"omitempty"`
		CaCertBase64      string        `json:"caCertBase64" jsonschema:"omitempty,format=base64"`

		// Support multiple certs, preserve the certbase64 and keybase64
		// for backward compatibility
		CertBase64 string `json:"certBase64" jsonschema:"omitempty,format=base64"`
		KeyBase64  string `json:"keyBase64" jsonschema:"omitempty,format=base64"`

		// Certs saved as map, key is domain name, value is cert
		Certs map[string]string `json:"certs" jsonschema:"omitempty"`
		// Keys saved as map, key is domain name, value is secret
		Keys map[string]string `json:"keys" jsonschema:"omitempty"`

		IPFilter *ipfilter.Spec `json:"ipFilter,omitempty" jsonschema:"omitempty"`
		Rules    []*Rule        `json:"rules" jsonschema:"omitempty"`

		GlobalFilter string `json:"globalFilter,omitempty" jsonschema:"omitempty"`
	}

	// Rule is first level entry of router.
	Rule struct {
		// NOTICE: If the field is a pointer, it must have `omitempty` in tag `json`
		// when it has `omitempty` in tag `jsonschema`.
		// Otherwise it will output null value, which is invalid in json schema (the type is object).
		// the original reason is the jsonscheme(genjs) has not support multiple types.
		// Reference: https://github.com/alecthomas/jsonschema/issues/30
		// In the future if we have the scenario where we need marshal the field, but omitempty
		// in the schema, we are suppose to support multiple types on our own.
		IPFilter   *ipfilter.Spec `json:"ipFilter,omitempty" jsonschema:"omitempty"`
		Host       string         `json:"host" jsonschema:"omitempty"`
		HostRegexp string         `json:"hostRegexp" jsonschema:"omitempty,format=regexp"`
		Paths      []*Path        `json:"paths" jsonschema:"omitempty"`
	}

	// Path is second level entry of router.
	Path struct {
		IPFilter          *ipfilter.Spec `json:"ipFilter,omitempty" jsonschema:"omitempty"`
		Path              string         `json:"path,omitempty" jsonschema:"omitempty,pattern=^/"`
		PathPrefix        string         `json:"pathPrefix,omitempty" jsonschema:"omitempty,pattern=^/"`
		PathRegexp        string         `json:"pathRegexp,omitempty" jsonschema:"omitempty,format=regexp"`
		RewriteTarget     string         `json:"rewriteTarget" jsonschema:"omitempty"`
		Methods           []string       `json:"methods,omitempty" jsonschema:"omitempty,uniqueItems=true,format=httpmethod-array"`
		Backend           string         `json:"backend" jsonschema:"required"`
		Headers           Headers        `json:"headers" jsonschema:"omitempty"`
		ClientMaxBodySize int64          `json:"clientMaxBodySize" jsonschema:"omitempty"`
		MatchAllHeader    bool           `json:"matchAllHeader" jsonschema:"omitempty"`
		Queries           Queries        `json:"queries,omitempty" jsonschema:"omitempty"`
	}

	Headers []*Header
	Queries []*Query

	// Header is the third level entry of router. A header entry is always under a specific path entry, that is to mean
	// the headers entry will only be checked after a path entry matched. However, the headers entry has a higher priority
	// than the path entry itself.
	Header struct {
		Key    string   `json:"key" jsonschema:"required"`
		Values []string `json:"values,omitempty" jsonschema:"omitempty,uniqueItems=true"`
		Regexp string   `json:"regexp,omitempty" jsonschema:"omitempty,format=regexp"`

		re *regexp.Regexp
	}

	// Query is the third level entry
	Query struct {
		Key    string   `json:"key" jsonschema:"required"`
		Values []string `json:"values,omitempty" jsonschema:"omitempty,uniqueItems=true"`
		Regexp string   `json:"regexp,omitempty" jsonschema:"omitempty,format=regexp"`

		re *regexp.Regexp
	}
)

// Validate validates HTTPServerSpec.
func (spec *Spec) Validate() error {
	if !spec.HTTPS {
		if spec.HTTP3 {
			return fmt.Errorf("https is disabled when http3 enabled")
		}
		return nil
	}

	if spec.CertBase64 == "" && spec.KeyBase64 == "" && len(spec.Certs) == 0 && len(spec.Keys) == 0 && !spec.AutoCert {
		return fmt.Errorf("certBase64/keyBase64, certs/keys are both empty and autocert is disabled when https enabled")
	}
	_, err := spec.tlsConfig()
	return err
}

func tryDecodeBase64Pem(pem string) []byte {
	// The pem could in base64 encoding or plain text. It starts with '-' if it is
	// in plain text, and '-' is not a valid character in standard base64 encoding.
	// We first try to decode it as base64, and fallback to plain text if failed.
	d, err := base64.StdEncoding.DecodeString(pem)
	if err == nil {
		return d
	}
	return []byte(pem)
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
		secret, exists := spec.Keys[k]
		if !exists {
			return nil, fmt.Errorf("certs %s hasn't secret corresponded to it", k)
		}

		certPem := tryDecodeBase64Pem(v)
		keyPem := tryDecodeBase64Pem(secret)
		cert, err := tls.X509KeyPair(certPem, keyPem)
		if err != nil {
			return nil, fmt.Errorf("generate x509 key pair for %s failed: %s ", k, err)
		}
		certificates = append(certificates, cert)
	}

	if len(certificates) == 0 && !spec.AutoCert {
		return nil, fmt.Errorf("none valid certs and secret")
	}

	// TLS-ALPN-01 challenges requires HTTP server to listen on port 443, but we don't
	// know which HTTP server listen on this port (consider there's an nginx sitting in
	// front of Easegress), so all HTTP servers need to handle TLS-ALPN-01 challenges.
	// But for HTTP servers who have disabled AutoCert, it should only handle the
	// TLS-ALPN-01 token certificate request.
	tlsConf := &tls.Config{
		Certificates: certificates,
		NextProtos:   []string{"acme-tls/1"},
	}
	tlsConf.GetCertificate = func(chi *tls.ClientHelloInfo) (*tls.Certificate, error) {
		return autocertmanager.GetCertificate(chi, !spec.AutoCert /* tokenOnly */)
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

// Validate validates Path.
func (p *Path) Validate() error {
	if (stringtool.IsAllEmpty(p.Path, p.PathPrefix, p.PathRegexp)) && p.RewriteTarget != "" {
		return fmt.Errorf("rewriteTarget is specified but path is empty")
	}

	return nil
}

func (hs Headers) init() {
	for _, h := range hs {
		if h.Regexp != "" {
			h.re = regexp.MustCompile(h.Regexp)
		}
	}
}

func (hs Headers) Validate() error {
	for _, h := range hs {
		if len(h.Values) == 0 && h.Regexp == "" {
			return fmt.Errorf("both of values and regexp are empty for key: %s", h.Key)
		}
	}
	return nil
}

func (hs Headers) match(headers http.Header, matchAll bool) bool {
	if len(hs) == 0 {
		return true
	}

	if matchAll {
		for _, h := range hs {
			v := headers.Get(h.Key)
			if len(h.Values) > 0 && !stringtool.StrInSlice(v, h.Values) {
				return false
			}

			if h.Regexp != "" && !h.re.MatchString(v) {
				return false
			}
		}
	} else {
		for _, h := range hs {
			v := headers.Get(h.Key)
			if stringtool.StrInSlice(v, h.Values) {
				return true
			}

			if h.Regexp != "" && h.re.MatchString(v) {
				return true
			}
		}
	}

	return matchAll
}

func (qs Queries) init() {
	for _, q := range qs {
		if q.Regexp != "" {
			q.re = regexp.MustCompile(q.Regexp)
		}
	}
}

func (qs Queries) Validate() error {
	for _, q := range qs {
		if len(q.Values) == 0 && q.Regexp == "" {
			return fmt.Errorf("both of values and regexp are empty for key: %s", q.Key)
		}
	}
	return nil
}

func (qs Queries) match(query url.Values, matchAll bool) bool {
	if len(qs) == 0 {
		return true
	}

	if matchAll {
		for _, q := range qs {
			v := query.Get(q.Key)
			if len(q.Values) > 0 && !stringtool.StrInSlice(v, q.Values) {
				return false
			}

			if q.Regexp != "" && !q.re.MatchString(v) {
				return false
			}
		}
	} else {
		for _, q := range qs {
			v := query.Get(q.Key)
			if stringtool.StrInSlice(v, q.Values) {
				return true
			}

			if q.Regexp != "" && q.re.MatchString(v) {
				return true
			}
		}
	}

	return matchAll
}
