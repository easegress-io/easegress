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

package httpserver

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"

	"github.com/megaease/easegress/v2/pkg/object/autocertmanager"
	"github.com/megaease/easegress/v2/pkg/object/httpserver/routers"
	"github.com/megaease/easegress/v2/pkg/tracing"
	"github.com/megaease/easegress/v2/pkg/util/ipfilter"
)

type (
	// Spec describes the HTTPServer.
	Spec struct {
		HTTP3             bool          `json:"http3,omitempty"`
		KeepAlive         bool          `json:"keepAlive" jsonschema:"required"`
		HTTPS             bool          `json:"https" jsonschema:"required"`
		AutoCert          bool          `json:"autoCert,omitempty"`
		XForwardedFor     bool          `json:"xForwardedFor,omitempty"`
		Address           string        `json:"address,omitempty"`
		Port              uint16        `json:"port" jsonschema:"required,minimum=1"`
		ClientMaxBodySize int64         `json:"clientMaxBodySize,omitempty"`
		KeepAliveTimeout  string        `json:"keepAliveTimeout,omitempty" jsonschema:"format=duration"`
		MaxConnections    uint32        `json:"maxConnections,omitempty" jsonschema:"minimum=1"`
		CacheSize         uint32        `json:"cacheSize,omitempty"`
		Tracing           *tracing.Spec `json:"tracing,omitempty"`
		CaCertBase64      string        `json:"caCertBase64,omitempty" jsonschema:"format=base64"`

		// Support multiple certs, preserve the certbase64 and keybase64
		// for backward compatibility
		CertBase64 string `json:"certBase64,omitempty" jsonschema:"format=base64"`
		KeyBase64  string `json:"keyBase64,omitempty" jsonschema:"format=base64"`

		// Certs saved as map, key is domain name, value is cert
		Certs map[string]string `json:"certs,omitempty"`
		// Keys saved as map, key is domain name, value is secret
		Keys map[string]string `json:"keys,omitempty"`

		RouterKind string `json:"routerKind,omitempty" jsonschema:"enum=,enum=Ordered,enum=RadixTree"`

		IPFilter *ipfilter.Spec `json:"ipFilter,omitempty"`
		Rules    routers.Rules  `json:"rules,omitempty"`

		GlobalFilter string `json:"globalFilter,omitempty"`

		AccessLogFormat string `json:"accessLogFormat,omitempty"`
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

	if spec.AutoCert {
		tlsConf.GetCertificate = func(chi *tls.ClientHelloInfo) (*tls.Certificate, error) {
			// NOTE: Need to get the latest.
			acm, exists := autocertmanager.GetGlobalAutoCertManager()
			if !exists {
				return nil, fmt.Errorf("there is no AutoCertManager")
			}

			return acm.GetCertificate(chi, !spec.AutoCert /* tokenOnly */)
		}
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
