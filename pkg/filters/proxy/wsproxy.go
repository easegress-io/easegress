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
	"github.com/megaease/easegress/pkg/supervisor"
)

const (
	// WebSocketProxyKind is the kind of WebSocketProxy.
	WebSocketProxyKind = "WebSocketProxy"

	/*
		resultInternalError = "internalError"
		resultClientError   = "clientError"
		resultServerError   = "serverError"
		resultFailureCode   = "failureCode"

		// result for resilience
		resultTimeout        = "timeout"
		resultShortCircuited = "shortCircuited"
	*/
)

var kindWebSocketProxy = &filters.Kind{
	Name:        WebSocketProxyKind,
	Description: "WebSocketProxy is the proxy for web sockets",
	Results: []string{
		resultInternalError,
		resultClientError,
		resultServerError,
		resultFailureCode,
		resultTimeout,
	},
	DefaultSpec: func() filters.Spec {
		return &Spec{
			MaxIdleConns:        10240,
			MaxIdleConnsPerHost: 1024,
		}
	},
	CreateInstance: func(spec filters.Spec) filters.Filter {
		return &WebSocketProxy{
			super: spec.Super(),
			spec:  spec.(*WebSocketProxySpec),
		}
	},
}

var _ filters.Filter = (*WebSocketProxy)(nil)

func init() {
	filters.Register(kindWebSocketProxy)
}

type (
	// WebSocketProxy is the filter WebSocketProxy.
	WebSocketProxy struct {
		super *supervisor.Supervisor
		spec  *WebSocketProxySpec

		mainPool       *WebSocketServerPool
		candidatePools []*WebSocketServerPool

		client *http.Client
	}

	// WebSocketProxySpec describes the WebSocketProxy.
	WebSocketProxySpec struct {
		filters.BaseSpec `json:",inline"`

		Pools               []*WebSocketServerPoolSpec `json:"pools" jsonschema:"required"`
		MTLS                *MTLS                      `json:"mtls,omitempty" jsonschema:"omitempty"`
		MaxIdleConns        int                        `json:"maxIdleConns" jsonschema:"omitempty"`
		MaxIdleConnsPerHost int                        `json:"maxIdleConnsPerHost" jsonschema:"omitempty"`
	}
)

// Validate validates Spec.
func (s *WebSocketProxySpec) Validate() error {
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

	return nil
}

// Name returns the name of the WebSocketProxy filter instance.
func (p *WebSocketProxy) Name() string {
	return p.spec.Name()
}

// Kind returns the kind of WebSocketProxy.
func (p *WebSocketProxy) Kind() *filters.Kind {
	return kind
}

// Spec returns the spec used by the WebSocketProxy
func (p *WebSocketProxy) Spec() filters.Spec {
	return p.spec
}

// Init initializes WebSocketProxy.
func (p *WebSocketProxy) Init() {
	p.reload()
}

// Inherit inherits previous generation of WebSocketProxy.
func (p *WebSocketProxy) Inherit(previousGeneration filters.Filter) {
	p.reload()
}

func (p *WebSocketProxy) tlsConfig() (*tls.Config, error) {
	mtls := p.spec.MTLS

	if mtls == nil {
		return &tls.Config{InsecureSkipVerify: true}, nil
	}

	certPem, _ := base64.StdEncoding.DecodeString(mtls.CertBase64)
	keyPem, _ := base64.StdEncoding.DecodeString(mtls.KeyBase64)
	cert, err := tls.X509KeyPair(certPem, keyPem)
	if err != nil {
		logger.Errorf("websocketproxy generates x509 key pair failed: %v", err)
		return &tls.Config{InsecureSkipVerify: true}, err
	}

	rootCertPem, _ := base64.StdEncoding.DecodeString(mtls.RootCertBase64)
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(rootCertPem)

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      caCertPool,
	}, nil
}

func (p *WebSocketProxy) reload() {
	for _, spec := range p.spec.Pools {
		name := ""
		if spec.Filter == nil {
			name = fmt.Sprintf("websocketproxy#%s#main", p.Name())
		} else {
			id := len(p.candidatePools)
			name = fmt.Sprintf("websocketproxy#%s#candidate#%d", p.Name(), id)
		}

		pool := NewWebSocketServerPool(p, spec, name)

		if spec.Filter == nil {
			p.mainPool = pool
		} else {
			p.candidatePools = append(p.candidatePools, pool)
		}
	}

	tlsCfg, _ := p.tlsConfig()
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
			TLSClientConfig:    tlsCfg,
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

// Status returns WebSocketProxy status.
func (p *WebSocketProxy) Status() interface{} {
	s := &Status{
		MainPool: p.mainPool.status(),
	}

	for _, pool := range p.candidatePools {
		s.CandidatePools = append(s.CandidatePools, pool.status())
	}

	return s
}

// Close closes WebSocketProxy.
func (p *WebSocketProxy) Close() {
	p.mainPool.close()

	for _, v := range p.candidatePools {
		v.close()
	}
}

// Handle handles HTTPContext.
func (p *WebSocketProxy) Handle(ctx *context.Context) (result string) {
	req := ctx.GetInputRequest().(*httpprot.Request)

	sp := p.mainPool
	for _, v := range p.candidatePools {
		if v.filter.Match(req) {
			sp = v
			break
		}
	}

	return sp.handle(ctx)
}
