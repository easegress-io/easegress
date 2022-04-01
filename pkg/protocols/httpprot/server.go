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

package httpprot

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/protocols"
)

// Server implements protocols.Server for HTTP.
type Server struct {
	spec   *ServerSpec
	target *ServerTarget

	client *http.Client
}

var fnSendRequest = func(r *http.Request, client *http.Client) (*http.Response, error) {
	return client.Do(r)
}

type ServerSpec struct {
	Server              *ServerTarget `yaml:"server" jsonschema:"required"`
	MTLS                *ServerMTLS   `yaml:"mtls,omitempty" jsonschema:"omitempty"`
	MaxIdleConns        int           `yaml:"maxIdleConns" jsonschema:"omitempty"`
	MaxIdleConnsPerHost int           `yaml:"maxIdleConnsPerHost" jsonschema:"omitempty"`
}

type ServerMTLS struct {
	CertBase64     string `yaml:"certBase64" jsonschema:"required,format=base64"`
	KeyBase64      string `yaml:"keyBase64" jsonschema:"required,format=base64"`
	RootCertBase64 string `yaml:"rootCertBase64" jsonschema:"required,format=base64"`
}

var _ protocols.Server = (*Server)(nil)

func (s *Server) Weight() int {
	return s.target.Weight
}

func (s *Server) tlsConfig() *tls.Config {
	if s.spec.MTLS == nil {
		return &tls.Config{
			InsecureSkipVerify: true,
		}
	}
	rootCertPem, _ := base64.StdEncoding.DecodeString(s.spec.MTLS.RootCertBase64)
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(rootCertPem)

	var certificates []tls.Certificate
	certPem, _ := base64.StdEncoding.DecodeString(s.spec.MTLS.CertBase64)
	keyPem, _ := base64.StdEncoding.DecodeString(s.spec.MTLS.KeyBase64)
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

func NewServer(spec interface{}) (protocols.Server, error) {
	server := &Server{}

	server.spec = spec.(*ServerSpec)
	server.target = server.spec.Server
	server.target.init()
	server.client = &http.Client{
		// NOTE: Timeout could be no limit, real client or server could cancel it.
		Timeout: 0,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 60 * time.Second,
				DualStack: true,
			}).DialContext,
			TLSClientConfig:    server.tlsConfig(),
			DisableCompression: false,
			// NOTE: The large number of Idle Connections can
			// reduce overhead of building connections.
			MaxIdleConns:          server.spec.MaxIdleConns,
			MaxIdleConnsPerHost:   server.spec.MaxIdleConnsPerHost,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	return server, nil
}

// SendRequest sends request to the server and returns the response.
func (s *Server) SendRequest(req protocols.Request) (protocols.Response, error) {
	resp, err := s.sendRequest(req.(*Request))
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (s *Server) sendRequest(req *Request) (*Response, error) {
	r, err := prepareRequest(req, s.target)
	if err != nil {
		return nil, err
	}
	resp, err := fnSendRequest(r, s.client)
	if err != nil {
		return nil, err
	}
	return NewResponse(resp), nil
}

func (s *Server) Close() error {
	return nil
}

func prepareRequest(req *Request, server *ServerTarget) (*http.Request, error) {
	url := server.URL + req.Path()
	if req.URL().RawQuery != "" {
		url += "?" + req.URL().RawQuery
	}

	stdr, err := http.NewRequestWithContext(req.Context(), req.Method(), url, req.GetPayload())
	if err != nil {
		return nil, fmt.Errorf("BUG: new request failed: %v", err)
	}

	stdr.Header = req.HTTPHeader()
	// only set host when server address is not host name.
	if !server.addrIsHostName {
		stdr.Host = req.Host()
	}
	return stdr, nil
}

type ServerTarget struct {
	URL            string   `yaml:"url" jsonschema:"required,format=url"`
	Tags           []string `yaml:"tags" jsonschema:"omitempty,uniqueItems=true"`
	Weight         int      `yaml:"weight" jsonschema:"omitempty,minimum=0,maximum=100"`
	addrIsHostName bool
}

func (s *ServerTarget) init() {
	u, err := url.Parse(s.URL)
	if err != nil {
		return
	}
	host := u.Host

	square := strings.LastIndexByte(host, ']')
	colon := strings.LastIndexByte(host, ':')

	// There is a port number, remove it.
	if colon > square {
		host = host[:colon]
	}

	// IPv6
	if square != -1 && host[0] == '[' {
		host = host[1:square]
	}

	s.addrIsHostName = net.ParseIP(host) == nil
}
