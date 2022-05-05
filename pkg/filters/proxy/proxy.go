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

package proxy

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/filters"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/protocols/httpprot"
	"github.com/megaease/easegress/pkg/resilience"
	"github.com/megaease/easegress/pkg/supervisor"
)

const (
	// Kind is the kind of Proxy.
	Kind = "Proxy"

	resultFallback      = "fallback"
	resultInternalError = "internalError"
	resultClientError   = "clientError"
	resultServerError   = "serverError"
	resultFailureCode   = "failureCode"

	// result for resilience
	resultTimeout        = "timeout"
	resultShortCircuited = "shortCircuited"
)

var kind = &filters.Kind{
	Name:        Kind,
	Description: "Proxy sets the proxy of proxy servers",
	Results: []string{
		resultFallback,
		resultInternalError,
		resultClientError,
		resultServerError,
		resultFailureCode,
		resultTimeout,
		resultShortCircuited,
	},
	DefaultSpec: func() filters.Spec {
		return &Spec{
			MaxIdleConns:        10240,
			MaxIdleConnsPerHost: 1024,
		}
	},
	CreateInstance: func(spec filters.Spec) filters.Filter {
		return &Proxy{
			super: spec.Super(),
			spec:  spec.(*Spec),
		}
	},
}

var _ filters.Filter = (*Proxy)(nil)
var _ filters.Resiliencer = (*Proxy)(nil)

func init() {
	filters.Register(kind)
}

var fnSendRequest = func(r *http.Request, client *http.Client) (*http.Response, error) {
	return client.Do(r)
}

type (
	// Proxy is the filter Proxy.
	Proxy struct {
		super *supervisor.Supervisor
		spec  *Spec

		mainPool       *ServerPool
		candidatePools []*ServerPool
		mirrorPool     *ServerPool

		client *http.Client

		compression *compression
	}

	// Spec describes the Proxy.
	Spec struct {
		filters.BaseSpec `yaml:",inline"`

		Pools               []*ServerPoolSpec `yaml:"pools" jsonschema:"required"`
		MirrorPool          *ServerPoolSpec   `yaml:"mirrorPool,omitempty" jsonschema:"omitempty"`
		FailureCodes        []int             `yaml:"failureCodes" jsonschema:"omitempty,uniqueItems=true,format=httpcode-array"`
		Compression         *CompressionSpec  `yaml:"compression,omitempty" jsonschema:"omitempty"`
		MTLS                *MTLS             `yaml:"mtls,omitempty" jsonschema:"omitempty"`
		MaxIdleConns        int               `yaml:"maxIdleConns" jsonschema:"omitempty"`
		MaxIdleConnsPerHost int               `yaml:"maxIdleConnsPerHost" jsonschema:"omitempty"`
	}

	// Status is the status of Proxy.
	Status struct {
		MainPool       *ServerPoolStatus   `yaml:"mainPool"`
		CandidatePools []*ServerPoolStatus `yaml:"candidatePools,omitempty"`
		MirrorPool     *ServerPoolStatus   `yaml:"mirrorPool,omitempty"`
	}

	// MTLS is the configuration for client side mTLS.
	MTLS struct {
		CertBase64     string `yaml:"certBase64" jsonschema:"required,format=base64"`
		KeyBase64      string `yaml:"keyBase64" jsonschema:"required,format=base64"`
		RootCertBase64 string `yaml:"rootCertBase64" jsonschema:"required,format=base64"`
	}
)

// Validate validates Spec.
func (s *Spec) Validate() error {
	numMainPool := 0
	for i, pool := range s.Pools {
		if pool.Filter == nil {
			numMainPool++
		}
		if err := pool.Validate(); err != nil {
			return fmt.Errorf("pool %d: %v", i, err)
		}
	}

	if numMainPool != 1 {
		return fmt.Errorf("one and only one mainPool is required")
	}

	if s.MirrorPool != nil {
		if s.MirrorPool.Filter == nil {
			return fmt.Errorf("filter of mirrorPool is required")
		}
		if s.MirrorPool.MemoryCache != nil {
			return fmt.Errorf("memoryCache must be empty in mirrorPool")
		}
	}

	return nil
}

// Name returns the name of the Proxy filter instance.
func (p *Proxy) Name() string {
	return p.spec.Name()
}

// Kind returns the kind of Proxy.
func (p *Proxy) Kind() *filters.Kind {
	return kind
}

// Spec returns the spec used by the Proxy
func (p *Proxy) Spec() filters.Spec {
	return p.spec
}

// Init initializes Proxy.
func (p *Proxy) Init() {
	p.reload()
}

// Inherit inherits previous generation of Proxy.
func (p *Proxy) Inherit(previousGeneration filters.Filter) {
	previousGeneration.Close()
	p.reload()
}

func (p *Proxy) tlsConfig() *tls.Config {
	mtls := p.spec.MTLS

	if mtls == nil {
		return &tls.Config{InsecureSkipVerify: true}
	}

	certPem, _ := base64.StdEncoding.DecodeString(mtls.CertBase64)
	keyPem, _ := base64.StdEncoding.DecodeString(mtls.KeyBase64)
	cert, err := tls.X509KeyPair(certPem, keyPem)
	if err != nil {
		logger.Errorf("proxy generates x509 key pair failed: %v", err)
		return &tls.Config{InsecureSkipVerify: true}
	}

	rootCertPem, _ := base64.StdEncoding.DecodeString(mtls.RootCertBase64)
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(rootCertPem)

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      caCertPool,
	}
}

func (p *Proxy) reload() {
	for _, spec := range p.spec.Pools {
		name := ""
		if spec.Filter == nil {
			name = fmt.Sprintf("proxy#%s#main", p.Name())
		} else {
			id := len(p.candidatePools)
			name = fmt.Sprintf("proxy#%s#candidate#%d", p.Name(), id)
		}

		pool := NewServerPool(p, spec, name)

		if spec.Filter == nil {
			p.mainPool = pool
		} else {
			p.candidatePools = append(p.candidatePools, pool)
		}
	}

	if p.spec.MirrorPool != nil {
		name := fmt.Sprintf("proxy#%s#mirror", p.Name())
		p.mirrorPool = NewServerPool(p, p.spec.MirrorPool, name)
	}

	if p.spec.Compression != nil {
		p.compression = newCompression(p.spec.Compression)
	}

	p.client = &http.Client{
		// NOTE: Timeout could be no limit, real client or server could cancel it.
		Timeout: 0,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 60 * time.Second,
				DualStack: true,
			}).DialContext,
			TLSClientConfig:    p.tlsConfig(),
			DisableCompression: false,
			// NOTE: The large number of Idle Connections can
			// reduce overhead of building connections.
			MaxIdleConns:          p.spec.MaxIdleConns,
			MaxIdleConnsPerHost:   p.spec.MaxIdleConnsPerHost,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// Status returns Proxy status.
func (p *Proxy) Status() interface{} {
	s := &Status{
		MainPool: p.mainPool.status(),
	}

	for _, pool := range p.candidatePools {
		s.CandidatePools = append(s.CandidatePools, pool.status())
	}

	if p.mirrorPool != nil {
		s.MirrorPool = p.mirrorPool.status()
	}

	return s
}

// Close closes Proxy.
func (p *Proxy) Close() {
	p.mainPool.close()

	for _, v := range p.candidatePools {
		v.close()
	}

	if p.mirrorPool != nil {
		p.mirrorPool.close()
	}
}

// Handle handles HTTPContext.
func (p *Proxy) Handle(ctx *context.Context) (result string) {
	req := ctx.Request().(*httpprot.Request)

	if p.mirrorPool != nil && p.mirrorPool.filter.Match(req) {
		go p.mirrorPool.handle(ctx, true)
	}

	sp := p.mainPool
	for _, v := range p.candidatePools {
		if v.filter.Match(req) {
			sp = v
			break
		}
	}

	return sp.handle(ctx, false)
}

// InjectResiliencePolicy injects resilience policies to the proxy.
func (p *Proxy) InjectResiliencePolicy(policies map[string]resilience.Policy) {
	p.mainPool.InjectResiliencePolicy(policies)

	for _, sp := range p.candidatePools {
		sp.InjectResiliencePolicy(policies)
	}
}
