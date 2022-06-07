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
	"sync/atomic"
	"time"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/httppipeline"
	"github.com/megaease/easegress/pkg/tracing"
	"github.com/megaease/easegress/pkg/util/fallback"
	zipkinhttp "github.com/openzipkin/zipkin-go/middleware/http"
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

var fnSendRequest = func(r *http.Request, client *Client) (*http.Response, error) {
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

		client atomic.Value //*Client

		compression *compression
	}

	// Client is a wrapper around http.Client.
	Client struct {
		client       *http.Client
		zipkinClient *zipkinhttp.Client
		tracing      *tracing.Tracing
	}

	// Spec describes the Proxy.
	Spec struct {
		Fallback            *FallbackSpec    `yaml:"fallback,omitempty" jsonschema:"omitempty"`
		MainPool            *PoolSpec        `yaml:"mainPool" jsonschema:"required"`
		CandidatePools      []*PoolSpec      `yaml:"candidatePools,omitempty" jsonschema:"omitempty"`
		MirrorPool          *PoolSpec        `yaml:"mirrorPool,omitempty" jsonschema:"omitempty"`
		FailureCodes        []int            `yaml:"failureCodes" jsonschema:"omitempty,uniqueItems=true,format=httpcode-array"`
		Compression         *CompressionSpec `yaml:"compression,omitempty" jsonschema:"omitempty"`
		MTLS                *MTLS            `yaml:"mtls,omitempty" jsonschema:"omitempty"`
		MaxIdleConns        int              `yaml:"maxIdleConns" jsonschema:"omitempty"`
		MaxIdleConnsPerHost int              `yaml:"maxIdleConnsPerHost" jsonschema:"omitempty"`
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

// NewClient creates a wrapper around http.Client
func NewClient(cl *http.Client, tr *tracing.Tracing) *Client {
	var zClient *zipkinhttp.Client
	if tr != nil && tr != tracing.NoopTracing {
		tracer := tr.Tracer
		zClient, _ = zipkinhttp.NewClient(tracer, zipkinhttp.WithClient(cl))
	}
	return &Client{client: cl, zipkinClient: zClient, tracing: tr}
}

// Do calls the correct http client
func (c *Client) Do(r *http.Request) (*http.Response, error) {
	if c.zipkinClient != nil {
		return c.zipkinClient.DoWithAppSpan(r, r.URL.Path)
	}
	return c.client.Do(r)
}

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
	return &Spec{
		MaxIdleConns:        10240,
		MaxIdleConnsPerHost: 1024,
	}
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

	b.client.Store(NewClient(b.createHTTPClient() /*tracing=*/, nil))
}

func (b *Proxy) createHTTPClient() *http.Client {
	return &http.Client{
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
			MaxIdleConns:          b.spec.MaxIdleConns,
			MaxIdleConnsPerHost:   b.spec.MaxIdleConnsPerHost,
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
// When we create new request for backend, we call http.NewRequestWithContext method and use context.Request().Body() as body.
// Based on golang std lib comments:
// https://github.com/golang/go/blob/95b68e1e02fa713719f02f6c59fb1532bd05e824/src/net/http/request.go#L856-L860
// If body is of type *bytes.Buffer, *bytes.Reader, or
// *strings.Reader, the returned request's ContentLength is set to its
// exact value (instead of -1), GetBody is populated (so 307 and 308
// redirects can replay the body), and Body is set to NoBody if the
// ContentLength is 0.
//
// So in this way, http.Request.ContentLength will be 0, and when http.Client send this request, it will delete
// "Content-Length" key in header. We solve this problem by set http.Request.ContentLength equal to
// http.Request.Header["Content-Length"] (if it is presented).
// Reading all context.Request().Body() and create new request with bytes.NewReader is another way, but it may cause performance loss.
//
// It is important that "Content-Length" in the Header is equal to the length of the Body. In easegress, when a filter change Request.Body,
// it will delete the header of "Content-Length". So, you should not worry about this when using our filters.
// But for customer filters, developer should make sure to delete or set "Context-Length" value in header when change Request.Body.
func (b *Proxy) Handle(ctx context.HTTPContext) (result string) {
	result = b.handle(ctx)
	return ctx.CallNextHandler(result)
}

func (b *Proxy) updateAndGetClient(tracingInstance *tracing.Tracing) *Client {
	client := b.client.Load().(*Client)
	if client.tracing == tracingInstance {
		return client
	}
	// tracingInstance is updated so recreate http.Client
	newClient := NewClient(b.createHTTPClient(), tracingInstance)
	b.client.Store(newClient)
	return newClient
}

func (b *Proxy) handle(ctx context.HTTPContext) (result string) {
	if b.mirrorPool != nil && b.mirrorPool.filter.Filter(ctx) {
		primaryBody, secondaryBody := newPrimarySecondaryReader(ctx.Request().Body())
		ctx.Request().SetBody(primaryBody, false)

		wg := &sync.WaitGroup{}
		wg.Add(1)
		defer wg.Wait()

		go func() {
			defer wg.Done()
			client := b.updateAndGetClient(ctx.Tracing())
			b.mirrorPool.handle(ctx, secondaryBody, client)
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

	client := b.updateAndGetClient(ctx.Tracing())
	result = p.handle(ctx, ctx.Request().Body(), client)
	if result != "" {
		if b.fallbackForCodes(ctx) {
			return resultFallback
		}
		return result
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
