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

package websocketserver

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/supervisor"
)

var (
	// defaultUpgrader specifies the parameters for upgrading an HTTP
	// connection to a WebSocket connection.
	defaultUpgrader = &websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}

	// defaultDialer is a dialer with all fields set to the default zero values.
	defaultDialer = websocket.DefaultDialer

	// defaultIntervalSecond is the default interval for polling websocket client and server.
	defaultIntervalSecond time.Duration = 1
)

// Proxy is an handler that takes an incoming WebSocket
// connection and proxies it to the backend server.
type Proxy struct {
	// server is the HTTPServer
	server    http.Server
	superSpec *supervisor.Spec

	// BackendURL URL is the
	backendURL *url.URL

	// Upgrader specifies the parameters for upgrading a incoming HTTP
	// connection to a WebSocket connection.
	upgrader *websocket.Upgrader

	//  Dialer contains options for connecting to the backend WebSocket server.
	dialer *websocket.Dialer

	done chan struct{}
}

// NewProxy returns a new Websocket proxy.
func newProxy(superSpec *supervisor.Spec) *Proxy {
	proxy := &Proxy{
		superSpec: superSpec,
		done:      make(chan struct{}),
	}
	go proxy.run()
	return proxy
}

// buildReq builds a URL with backend in spec and original HTTP request.
func (p *Proxy) buildReq(r *http.Request) *url.URL {
	u := *p.backendURL
	u.Fragment = r.URL.Fragment
	u.Path = r.URL.Path
	u.RawQuery = r.URL.RawQuery
	return &u
}

// passMsg passes websocket message from src to dst.
func (p *Proxy) passMsg(src, dst *websocket.Conn, errc chan error, stop chan struct{}) {
	handle := func() bool {
		msgType, msg, err := src.ReadMessage()
		if err != nil {
			m := websocket.FormatCloseMessage(websocket.CloseNormalClosure, fmt.Sprintf("%v", err))
			if e, ok := err.(*websocket.CloseError); ok {
				if e.Code != websocket.CloseNoStatusReceived {
					m = websocket.FormatCloseMessage(e.Code, e.Text)
				}
			}
			dst.WriteMessage(websocket.CloseMessage, m)
			errc <- err
			return false
		}
		err = dst.WriteMessage(msgType, msg)
		if err != nil {
			errc <- err
			return false
		}
		return true
	}

	for {
		select {
		// this request handling is stopped due to some error or websocketserver shutdown.
		case <-stop:
			return
		case <-time.After(defaultIntervalSecond * time.Second):
			if !handle() {
				return
			}
		}
	}
}

// run runs the websocket proxy.
func (p *Proxy) run() {
	spec := p.superSpec.ObjectSpec().(*Spec)
	backendURL, err := url.Parse(spec.Backend)
	if err != nil {
		logger.Errorf("invalid websocketserver backend URL: %s ", spec.Backend)
		return
	}

	p.backendURL = backendURL
	dialer := defaultDialer
	if strings.HasPrefix(spec.Backend, "wss") {
		tlsConfig, err := spec.wssTLSConfig()
		if err != nil {
			logger.Errorf("gen websocketserver backend tls failed: %v, spec :%#v", err, spec)
			return
		}
		dialer.TLSClientConfig = tlsConfig
	}
	p.dialer = dialer
	p.upgrader = defaultUpgrader

	http.HandleFunc("/", p.handle)
	addr := fmt.Sprintf(":%d", spec.Port)
	svr := &http.Server{
		Addr:    addr,
		Handler: nil,
	}

	if spec.HTTPS {
		tlsConfig, err := spec.tlsConfig()
		if err != nil {
			logger.Errorf("gen websocketserver's httpserver tlsConfig: %#v, failed: %v", spec, err)
		}
		svr.TLSConfig = tlsConfig
	}

	if err := svr.ListenAndServe(); err != nil {
		logger.Errorf("websocketserver ListenAndServe failed! err: %s\n", err.Error())
	}
}

// copyHeader copies headers from the incoming request to the dialer and forward them to
// the destinations.
func (p *Proxy) copyHeader(req *http.Request) http.Header {

	requestHeader := http.Header{}
	if origin := req.Header.Get("Origin"); origin != "" {
		requestHeader.Add("Origin", origin)
	}
	for _, prot := range req.Header[http.CanonicalHeaderKey("Sec-WebSocket-Protocol")] {
		requestHeader.Add("Sec-WebSocket-Protocol", prot)
	}
	for _, cookie := range req.Header[http.CanonicalHeaderKey("Cookie")] {
		requestHeader.Add("Cookie", cookie)
	}
	if req.Host != "" {
		requestHeader.Set("Host", req.Host)
	}

	if clientIP, _, err := net.SplitHostPort(req.RemoteAddr); err == nil {
		if prior, ok := req.Header["X-Forwarded-For"]; ok {
			clientIP = strings.Join(prior, ", ") + ", " + clientIP
		}
		requestHeader.Set("X-Forwarded-For", clientIP)
	}

	requestHeader.Set("X-Forwarded-Proto", "http")
	if req.TLS != nil {
		requestHeader.Set("X-Forwarded-Proto", "https")
	}

	return requestHeader
}

// upgradeRspHeader passes only selected headers as return.
func (p *Proxy) upgradeRspHeader(resp *http.Response) http.Header {
	upgradeHeader := http.Header{}
	if hdr := resp.Header.Get("Sec-Websocket-Protocol"); hdr != "" {
		upgradeHeader.Set("Sec-Websocket-Protocol", hdr)
	}
	if hdr := resp.Header.Get("Set-Cookie"); hdr != "" {
		upgradeHeader.Set("Set-Cookie", hdr)
	}
	return upgradeHeader
}

// handle implements the http.Handler that proxies WebSocket connections.
func (p *Proxy) handle(rw http.ResponseWriter, req *http.Request) {
	connBackend, resp, err := p.dialer.Dial(p.buildReq(req).String(), p.copyHeader(req))
	if err != nil {
		logger.Errorf("Proxy: couldn't dial to ws remote backend url: %s, err: %v", p.backendURL.String(), err)
		if resp != nil {
			// Handle WebSocket handshake failed scenario.
			// Should send back a non-nil *http.Response for callers to handle
			// `redirects`, `authentication` operations and so on.
			if err := copyResponse(rw, resp); err != nil {
				logger.Errorf("Proxy: couldn't write response after failed at remote backend: %s handshake: %v",
					p.backendURL.String(), err)
			}
		} else {
			http.Error(rw, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
		}
		return
	}
	defer connBackend.Close()

	// Upgrade the incoming request to a WebSocket connection(Protocol Switching).
	// Also pass the header from the Dial handshake.
	connClient, err := p.upgrader.Upgrade(rw, req, p.upgradeRspHeader(resp))
	if err != nil {
		logger.Errorf("proxy upgrade req: %#v failed: %s", req, err)
		return
	}
	defer connClient.Close()

	errClient := make(chan error, 1)
	errBackend := make(chan error, 1)
	stop := make(chan struct{})

	defer close(stop)

	// pass msg from backend to client
	go p.passMsg(connBackend, connClient, errBackend, stop)
	// pass msg from client to backend
	go p.passMsg(connClient, connBackend, errClient, stop)

	var errMsg string
	select {
	case err = <-errBackend:
		errMsg = "passing msg from backend: %s to client failed: %v"
	case err = <-errClient:
		errMsg = "passing msg client to backend: %s failed: %v"
	case <-p.done:
		logger.Debugf("shutdown websocketserver in request handling")
		return
	}

	if e, ok := err.(*websocket.CloseError); !ok || e.Code == websocket.CloseAbnormalClosure {
		logger.Errorf(errMsg, p.backendURL.String(), err)
	}
	// other error type is expected, not need to log
}

// Close close websocket proxy.
func (p *Proxy) Close() {
	close(p.done)

	ctx, cancelFunc := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelFunc()
	err := p.server.Shutdown(ctx)
	if err != nil {
		logger.Warnf("shutdown http server %s failed: %v",
			p.superSpec.Name(), err)
	}
}

func copyResponse(rw http.ResponseWriter, resp *http.Response) error {
	for k, vv := range resp.Header {
		for _, v := range vv {
			rw.Header().Add(k, v)
		}
	}
	rw.WriteHeader(resp.StatusCode)
	defer resp.Body.Close()

	_, err := io.Copy(rw, resp.Body)
	return err
}
