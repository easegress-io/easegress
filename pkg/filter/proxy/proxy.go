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
	"sync"
	"time"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/httppipeline"
	"github.com/megaease/easegress/pkg/util/fallback"
)

const (
	// Kind is the kind of Proxy.
	Kind = "Proxy"

	resultFallback      = "fallback"
	resultInternalError = "internalError"
	resultClientError   = "clientError"
	resultServerError   = "serverError"
)

var results = []string{
	resultFallback,
	resultInternalError,
	resultClientError,
	resultServerError,
}

func init() {
	httppipeline.Register(&Proxy{})
}

var fnSendRequest = func(r *http.Request, client *http.Client) (*http.Response, error) {
	return client.Do(r)
}

type (
	// Proxy is the filter Proxy.
	Proxy struct {
		filterSpec *httppipeline.FilterSpec
		spec       *Spec

		fallback *fallback.Fallback

		mainPool       *pool
		candidatePools []*pool
		mirrorPool     *pool

		client *http.Client

		compression *compression
	}

	// Spec describes the Proxy.
	Spec struct {
		Fallback       *FallbackSpec    `yaml:"fallback,omitempty" jsonschema:"omitempty"`
		MainPool       *PoolSpec        `yaml:"mainPool" jsonschema:"required"`
		CandidatePools []*PoolSpec      `yaml:"candidatePools,omitempty" jsonschema:"omitempty"`
		MirrorPool     *PoolSpec        `yaml:"mirrorPool,omitempty" jsonschema:"omitempty"`
		FailureCodes   []int            `yaml:"failureCodes" jsonschema:"omitempty,uniqueItems=true,format=httpcode-array"`
		Compression    *CompressionSpec `yaml:"compression,omitempty" jsonschema:"omitempty"`
		MTLS           *MTLS            `yaml:"mtls,omitempty" jsonschema:"omitempty"`
	}

	// FallbackSpec describes the fallback policy.
	FallbackSpec struct {
		ForCodes      bool `yaml:"forCodes"`
		fallback.Spec `yaml:",inline"`
	}

	// Status is the status of Proxy.
	Status struct {
		MainPool       *PoolStatus   `yaml:"mainPool"`
		CandidatePools []*PoolStatus `yaml:"candidatePools,omitempty"`
		MirrorPool     *PoolStatus   `yaml:"mirrorPool,omitempty"`
	}

	// MTLS is the configuration for client side mTLS.
	MTLS struct {
		CertBase64     string `yaml:"certBase64" jsonschema:"required,format=base64"`
		KeyBase64      string `yaml:"keyBase64" jsonschema:"required,format=base64"`
		RootCertBase64 string `yaml:"rootCertBase64" jsonschema:"required,format=base64"`
	}
)

// Validate validates Spec.
func (s Spec) Validate() error {
	// NOTE: The tag of v parent may be behind mainPool.
	if s.MainPool == nil {
		return fmt.Errorf("mainPool is required")
	}

	if s.MainPool.Filter != nil {
		return fmt.Errorf("filter must be empty in mainPool")
	}

	if len(s.CandidatePools) > 0 {
		for _, v := range s.CandidatePools {
			if v.Filter == nil {
				return fmt.Errorf("filter of candidatePool is required")
			}
		}
	}

	if s.MirrorPool != nil {
		if s.MirrorPool.Filter == nil {
			return fmt.Errorf("filter of mirrorPool is required")
		}
		if s.MirrorPool.MemoryCache != nil {
			return fmt.Errorf("memoryCache must be empty in mirrorPool")
		}
	}

	if len(s.FailureCodes) == 0 {
		if s.Fallback != nil {
			return fmt.Errorf("fallback needs failureCodes")
		}
	}

	return nil
}

// Kind returns the kind of Proxy.
func (b *Proxy) Kind() string {
	return Kind
}

// DefaultSpec returns the default spec of Proxy.
func (b *Proxy) DefaultSpec() interface{} {
	return &Spec{}
}

// Description returns the description of Proxy.
func (b *Proxy) Description() string {
	return "Proxy sets the proxy of proxy servers"
}

// Results returns the results of Proxy.
func (b *Proxy) Results() []string {
	return results
}

// Init initializes Proxy.
func (b *Proxy) Init(filterSpec *httppipeline.FilterSpec) {
	b.filterSpec, b.spec = filterSpec, filterSpec.FilterSpec().(*Spec)
	b.reload()
}

// Inherit inherits previous generation of Proxy.
func (b *Proxy) Inherit(filterSpec *httppipeline.FilterSpec, previousGeneration httppipeline.Filter) {
	previousGeneration.Close()
	b.Init(filterSpec)
}

func (b *Proxy) needmTLS() bool {
	return b.spec.MTLS != nil
}

func (b *Proxy) tlsConfig() *tls.Config {
	if !b.needmTLS() {
		return &tls.Config{
			InsecureSkipVerify: true,
		}
	}
	rootCertPem, _ := base64.StdEncoding.DecodeString(b.spec.MTLS.RootCertBase64)
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(rootCertPem)

	var certificates []tls.Certificate
	certPem, _ := base64.StdEncoding.DecodeString(b.spec.MTLS.CertBase64)
	keyPem, _ := base64.StdEncoding.DecodeString(b.spec.MTLS.KeyBase64)
	cert, err := tls.X509KeyPair(certPem, keyPem)
	if err != nil {
		logger.Errorf("proxy generates x509 key pair failed: %v", err)
		return &tls.Config{
			InsecureSkipVerify: true,
		}
	}
	certificates = append(certificates, cert)
	return &tls.Config{
		Certificates: certificates,
		RootCAs:      caCertPool,
	}
}

func (b *Proxy) reload() {
	super := b.filterSpec.Super()

	b.mainPool = newPool(super, b.spec.MainPool, "proxy#main",
		true /*writeResponse*/, b.spec.FailureCodes)

	if b.spec.Fallback != nil {
		b.fallback = fallback.New(&b.spec.Fallback.Spec)
	}

	if len(b.spec.CandidatePools) > 0 {
		var candidatePools []*pool
		for k := range b.spec.CandidatePools {
			candidatePools = append(candidatePools,
				newPool(super, b.spec.CandidatePools[k], fmt.Sprintf("proxy#candidate#%d", k),
					true, b.spec.FailureCodes))
		}
		b.candidatePools = candidatePools
	}
	if b.spec.MirrorPool != nil {
		b.mirrorPool = newPool(super, b.spec.MirrorPool, "proxy#mirror",
			false /*writeResponse*/, b.spec.FailureCodes)
	}

	if b.spec.Compression != nil {
		b.compression = newCompression(b.spec.Compression)
	}

	b.client = &http.Client{
		// NOTE: Timeout could be no limit, real client or server could cancel it.
		Timeout: 0,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 60 * time.Second,
				DualStack: true,
			}).DialContext,
			TLSClientConfig:    b.tlsConfig(),
			DisableCompression: false,
			// NOTE: The large number of Idle Connections can
			// reduce overhead of building connections.
			MaxIdleConns:          10240,
			MaxIdleConnsPerHost:   512,
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
func (b *Proxy) Status() interface{} {
	s := &Status{
		MainPool: b.mainPool.status(),
	}
	if b.candidatePools != nil {
		for k := range b.candidatePools {
			s.CandidatePools = append(s.CandidatePools, b.candidatePools[k].status())
		}
	}
	if b.mirrorPool != nil {
		s.MirrorPool = b.mirrorPool.status()
	}
	return s
}

// Close closes Proxy.
func (b *Proxy) Close() {
	b.mainPool.close()

	if b.candidatePools != nil {
		for _, v := range b.candidatePools {
			v.close()
		}
	}

	if b.mirrorPool != nil {
		b.mirrorPool.close()
	}
}

func (b *Proxy) fallbackForCodes(ctx context.HTTPContext) bool {
	if b.fallback != nil && b.spec.Fallback.ForCodes {
		for _, code := range b.spec.FailureCodes {
			if ctx.Response().StatusCode() == code {
				b.fallback.Fallback(ctx)
				return true
			}
		}
	}
	return false
}

// Handle handles HTTPContext.
func (b *Proxy) Handle(ctx context.HTTPContext) (result string) {
	result = b.handle(ctx)
	return ctx.CallNextHandler(result)
}

func (b *Proxy) handle(ctx context.HTTPContext) (result string) {
	if b.mirrorPool != nil && b.mirrorPool.filter.Filter(ctx) {
		master, slave := newMasterSlaveReader(ctx.Request().Body())
		ctx.Request().SetBody(master)

		wg := &sync.WaitGroup{}
		wg.Add(1)
		defer wg.Wait()

		go func() {
			defer wg.Done()
			b.mirrorPool.handle(ctx, slave, b.client)
		}()
	}

	var p *pool
	if len(b.candidatePools) > 0 {
		for k, v := range b.candidatePools {
			if v.filter.Filter(ctx) {
				p = b.candidatePools[k]
				break
			}
		}
	}

	if p == nil {
		p = b.mainPool
	}

	if p.memoryCache != nil && p.memoryCache.Load(ctx) {
		return ""
	}

	result = p.handle(ctx, ctx.Request().Body(), b.client)
	if result != "" {
		return result
	}

	if b.fallbackForCodes(ctx) {
		return resultFallback
	}

	// compression and memoryCache only work for
	// normal traffic from real proxy servers.
	if b.compression != nil {
		b.compression.compress(ctx)
	}

	if p.memoryCache != nil {
		p.memoryCache.Store(ctx)
	}

	return ""
}
