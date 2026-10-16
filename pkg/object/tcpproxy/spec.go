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

package tcpproxy

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
)

const (
	// LoadBalanceRoundRobin is the round-robin load balance strategy.
	LoadBalanceRoundRobin = "roundRobin"
	// LoadBalanceWeightedRoundRobin is the weighted round-robin load balance strategy.
	LoadBalanceWeightedRoundRobin = "weightedRoundRobin"
)

type (
	// Backend is a backend server for TCP proxy.
	Backend struct {
		// Address is the address of the backend, e.g. "127.0.0.1:8080".
		Address string `json:"address" jsonschema:"required"`
		// Weight is the weight for weighted round-robin. Default is 1.
		Weight uint32 `json:"weight,omitempty" jsonschema:"minimum=1"`
	}

	// Spec describes the TCPProxy.
	Spec struct {
		// Address is the address to listen on. Default is "".
		Address string `json:"address,omitempty"`
		// Port is the port to listen on.
		Port uint16 `json:"port" jsonschema:"required,minimum=1"`
		// MaxConnections is the maximum number of concurrent connections.
		MaxConnections uint32 `json:"maxConnections,omitempty" jsonschema:"minimum=1"`

		// TLS defines TLS termination configuration.
		TLS *TLSSpec `json:"tls,omitempty"`

		// ConnectTimeout is the timeout for connecting to backend servers.
		// Format is duration, e.g. "10s". Default is "10s".
		ConnectTimeout string `json:"connectTimeout,omitempty" jsonschema:"format=duration"`
		// SendTimeout is the timeout for sending data to backend servers.
		SendTimeout string `json:"sendTimeout,omitempty" jsonschema:"format=duration"`
		// ReceiveTimeout is the timeout for receiving data from backend servers.
		ReceiveTimeout string `json:"receiveTimeout,omitempty" jsonschema:"format=duration"`
		// IdleTimeout is the timeout for idle connections.
		IdleTimeout string `json:"idleTimeout,omitempty" jsonschema:"format=duration"`

		// LoadBalance is the load balance strategy.
		// Supported values: "roundRobin", "weightedRoundRobin".
		// Default is "roundRobin".
		LoadBalance string `json:"loadBalance,omitempty" jsonschema:"enum=,enum=roundRobin,enum=weightedRoundRobin"`

		// Backends is the list of backend servers.
		Backends []*Backend `json:"backends" jsonschema:"required,minItems=1"`
	}

	// TLSSpec defines TLS termination configuration.
	TLSSpec struct {
		// Enable controls whether TLS termination is enabled.
		Enable bool `json:"enable,omitempty"`

		// Support multiple certs, preserve the certBase64 and keyBase64
		// for backward compatibility
		CertBase64 string `json:"certBase64,omitempty" jsonschema:"format=base64"`
		KeyBase64  string `json:"keyBase64,omitempty" jsonschema:"format=base64"`

		// Certs saved as map, key is domain name, value is cert
		Certs map[string]string `json:"certs,omitempty"`
		// Keys saved as map, key is domain name, value is secret
		Keys map[string]string `json:"keys,omitempty"`

		// CaCertBase64 is the base64 encoded CA certificate for client authentication.
		CaCertBase64 string `json:"caCertBase64,omitempty" jsonschema:"format=base64"`
	}
)

func tryDecodeBase64Pem(pem string) []byte {
	d, err := base64.StdEncoding.DecodeString(pem)
	if err == nil {
		return d
	}
	return []byte(pem)
}

// Validate validates Spec.
func (spec *Spec) Validate() error {
	if spec.Port == 0 {
		return fmt.Errorf("port is required")
	}

	if len(spec.Backends) == 0 {
		return fmt.Errorf("backends is required and must have at least one entry")
	}

	for i, b := range spec.Backends {
		if b.Address == "" {
			return fmt.Errorf("backends[%d].address is required", i)
		}
	}

	switch spec.LoadBalance {
	case LoadBalanceRoundRobin, LoadBalanceWeightedRoundRobin, "":
	default:
		return fmt.Errorf("unsupported load balance strategy: %s", spec.LoadBalance)
	}

	if spec.TLS != nil {
		if spec.TLS.CertBase64 == "" && spec.TLS.KeyBase64 == "" &&
			len(spec.TLS.Certs) == 0 && len(spec.TLS.Keys) == 0 {
			return fmt.Errorf("TLS certBase64/keyBase64, certs/keys are both empty")
		}
		if _, err := spec.TLS.tlsConfig(); err != nil {
			return fmt.Errorf("invalid TLS config: %v", err)
		}
	}

	return nil
}

// tlsConfig builds tls.Config from TLSSpec.
func (t *TLSSpec) tlsConfig() (*tls.Config, error) {
	var certificates []tls.Certificate

	if t.CertBase64 != "" && t.KeyBase64 != "" {
		certPem := tryDecodeBase64Pem(t.CertBase64)
		keyPem := tryDecodeBase64Pem(t.KeyBase64)
		cert, err := tls.X509KeyPair(certPem, keyPem)
		if err != nil {
			return nil, fmt.Errorf("generate x509 key pair failed: %v", err)
		}
		certificates = append(certificates, cert)
	}

	for k, v := range t.Certs {
		secret, exists := t.Keys[k]
		if !exists {
			return nil, fmt.Errorf("certs %s hasn't secret corresponded to it", k)
		}

		certPem := tryDecodeBase64Pem(v)
		keyPem := tryDecodeBase64Pem(secret)
		cert, err := tls.X509KeyPair(certPem, keyPem)
		if err != nil {
			return nil, fmt.Errorf("generate x509 key pair for %s failed: %s", k, err)
		}
		certificates = append(certificates, cert)
	}

	if len(certificates) == 0 {
		return nil, fmt.Errorf("no valid certificates found")
	}

	tlsConf := &tls.Config{
		Certificates: certificates,
	}

	if len(t.CaCertBase64) != 0 {
		rootCertPem := tryDecodeBase64Pem(t.CaCertBase64)
		certPool := x509.NewCertPool()
		certPool.AppendCertsFromPEM(rootCertPem)

		tlsConf.ClientAuth = tls.RequireAndVerifyClientCert
		tlsConf.ClientCAs = certPool
	}

	return tlsConf, nil
}
