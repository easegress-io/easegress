package plugins

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/erikdubbelboer/fasthttp"
	"github.com/hexdecteam/easegateway-types/plugins"
	"github.com/hexdecteam/easegateway-types/task"

	"common"
	eghttp "http"
	"logger"
)

//// This file defines and implementes client side HTTP related interfaces

//// client side response interface, only supports get methods
type response interface {
	BodyReadCloser() plugins.SizedReadCloser
	Header() plugins.Header
	StatusCode() int
	Dump() (string, error)
}

// netHTTPResponse leverages net/http, implements response interface
type netHTTPResponse struct {
	r *http.Response
}

func (n *netHTTPResponse) BodyReadCloser() plugins.SizedReadCloser {
	return common.NewSizedReadCloser(n.r.Body, n.r.ContentLength)
}

func (n *netHTTPResponse) Header() plugins.Header {
	return &eghttp.NetResponseHeader{H: n.r.Header}
}

func (n *netHTTPResponse) StatusCode() int {
	return n.r.StatusCode
}

func (n *netHTTPResponse) Dump() (string, error) {
	if s, e := httputil.DumpResponse(n.r, true /* dump response body */); e != nil {
		return "", e
	} else {
		return common.B2s(s), nil
	}

}

// fastHTTPResponse leverages fasthttp, implements response interface
type fastHTTPResponse struct {
	r   *fasthttp.Response
	ctx *fasthttp.RequestCtx
}

func (f *fastHTTPResponse) BodyReadCloser() plugins.SizedReadCloser {
	r, w := io.Pipe()
	go func() {
		err := f.r.BodyWriteTo(w)
		w.CloseWithError(err)
		fasthttp.ReleaseResponse(f.r)
	}()
	return common.NewSizedReadCloser(r, int64(f.r.Header.ContentLength()))
}

func (f *fastHTTPResponse) Header() plugins.Header {
	return &eghttp.FastResponseHeader{
		H: &f.r.Header,
	}
}

func (f *fastHTTPResponse) StatusCode() int {
	return f.r.StatusCode()
}

func (f *fastHTTPResponse) Dump() (string, error) {
	r := fasthttp.AcquireResponse()
	// CopyTo copies req contents to dst except of body stream.
	// So this operation avoids drain request body stream
	f.r.CopyTo(r)
	// dump body
	body := f.r.Body()
	r.SetBody(body)
	s := r.String()
	fasthttp.ReleaseResponse(r)
	return s, nil
}

//// client side request interface
type request interface {
	Header() plugins.Header
	Do(task.Task) (response, time.Duration, error)
	Dump() (string, error)
}

// httpRequest leverages net/http, implements request interface
type httpRequest struct {
	req    *http.Request
	client *http.Client
}

func (h *httpRequest) Header() plugins.Header {
	return &eghttp.NetRequestHeader{
		R: h.req,
	}
}

func (h *httpRequest) Dump() (string, error) {
	if s, e := httputil.DumpRequest(h.req, eghttp.DumpBody); e != nil {
		return "", e
	} else {
		return common.B2s(s), nil
	}
}

func (h *httpRequest) Do(t task.Task) (response, time.Duration, error) {
	r := make(chan *http.Response)
	e := make(chan error)

	defer close(r)
	defer close(e)

	cancelCtx, cancel := context.WithCancel(context.Background())
	req := h.req.WithContext(cancelCtx)
	var requestStartAt time.Time
	go func() {
		defer func() {
			// channel e and r can be closed first before return by existing send()
			// caused by task cancellation, the result or error of Do() can be ignored safely.
			recover()
		}()

		requestStartAt = common.Now()
		resp, err := h.client.Do(req)
		if err != nil {
			e <- err
		} else {
			r <- resp
		}
	}()

	select {
	case resp := <-r:
		responseDuration := common.Since(requestStartAt)

		return &netHTTPResponse{resp}, responseDuration, nil
	case err := <-e:
		responseDuration := common.Since(requestStartAt)

		msg := err.Error()
		if m, err := url.QueryUnescape(msg); err == nil {
			msg = m
		}

		return nil, responseDuration, fmt.Errorf("%s", msg)
	case <-t.Cancel():
		cancel()
		responseDuration := common.Since(requestStartAt)
		// TODO(shengdong) add t.CancelCause() ?

		return nil, responseDuration, cancelledError
	}
}

// fastRequest leverages fasthttp, implements request interface
type fastRequest struct {
	req     *fasthttp.Request
	client  *fasthttp.Client
	timeout time.Duration
}

func (f *fastRequest) Header() plugins.Header {
	// currently it's ok to set isTLS to false here, because it's client side request
	// maybe set according to request url in the future
	return eghttp.NewFastRequestHeader(false /* isTLS */, f.req.URI(), &f.req.Header)
}

func (h *fastRequest) Dump() (string, error) {
	r := fasthttp.AcquireRequest()
	// CopyTo copies req contents to dst except of body stream.
	// So this operation avoids drain request body stream
	h.req.CopyTo(r)
	if eghttp.DumpBody {
		body := h.req.Body()
		r.SetBody(body)
	}

	s := r.String()
	fasthttp.ReleaseRequest(r)
	return s, nil
}

func (f *fastRequest) Do(t task.Task) (response, time.Duration, error) {
	// release request after we done request
	defer fasthttp.ReleaseRequest(f.req)
	// resp is Released in response.BodyReadCloser() if this function returns without error
	resp := fasthttp.AcquireResponse()

	errc := make(chan error)
	canceled := make(chan bool)

	var start time.Time
	go func() {
		start = common.Now()
		err := f.client.DoTimeout(f.req, resp, f.timeout)
		errc <- err
		select {
		case <-canceled:
			fasthttp.ReleaseResponse(resp)
		default:
		}
	}()

	select {
	case err := <-errc:
		if err != nil {
			fasthttp.ReleaseResponse(resp)
			return nil, common.Since(start), fmt.Errorf("fasthttp client.Do() failed: %v", err)
		}
		return &fastHTTPResponse{
			r: resp,
		}, common.Since(start), nil
	case <-t.Cancel():
		canceled <- true
		//  fasthttp doesn't support cancel http request, so just ignore the response
		return nil, common.Since(start), cancelledError
	}
}

//// http client interface
type client interface {
	CreateRequest(method, url string, bodyReader plugins.SizedReadCloser) (request, error)
}

// netHTTPClient leverages net/http, implements client interface
type netHTTPClient struct {
	c *http.Client
}

func (n *netHTTPClient) CreateRequest(method, url string, bodyReader plugins.SizedReadCloser) (request, error) {
	req, error := http.NewRequest(method, url, bodyReader)
	if error != nil {
		return nil, error
	}
	if req.URL.Path == "" {
		req.URL.Path = "/"
	} else {
		// FIXME: this operation may have side-effect:
		// For example: if PARAM is "" in url `/{PARAM}/`,
		// then that url will be cleaned up to `/`. This may lead bugs.
		req.URL.Path = common.RemoveRepeatedByte(req.URL.Path, '/')
	}

	req.ContentLength = bodyReader.Size()
	req.Host = req.Header.Get("Host") // https://github.com/golang/go/issues/7682
	return &httpRequest{req, n.c}, nil
}

// fastHTTPClient leverages fasthttp, implements client interface
type fastHTTPClient struct {
	c       *fasthttp.Client
	timeout time.Duration
}

func (n *fastHTTPClient) CreateRequest(method, url string, bodyReader plugins.SizedReadCloser) (request, error) {
	req := fasthttp.AcquireRequest()
	// fasthttp.URI will do path normalization innerly
	req.Header.SetMethod(method)
	req.Header.SetRequestURI(url)
	bodySize := int(bodyReader.Size())
	req.Header.SetContentLength(bodySize)

	// if bodySize < 0, then fasthttp will regard request transfer-encoding as chunked
	req.SetBodyStream(bodyReader, bodySize)

	return &fastRequest{req, n.c, n.timeout}, nil
}

// client factory, creates concrete client implementation
func createClient(c *HTTPOutputConfig) client {
	timeout := time.Duration(c.TimeoutSec) * time.Second
	tlsConfig := new(tls.Config)
	tlsConfig.InsecureSkipVerify = c.Insecure

	if c.cert != nil {
		tlsConfig.Certificates = []tls.Certificate{*c.cert}
		tlsConfig.BuildNameToCertificate()
	}

	if c.caCert != nil {
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(c.caCert)
		tlsConfig.RootCAs = caCertPool
	}

	keepAlivePeriod := c.ConnKeepAliveSec
	if c.connKeepAlive == 0 {
		keepAlivePeriod = 0 // disable keep-alive
	}
	maxIdleConns := 10240
	maxIdleConnsPerHost := 512
	if c.typ == eghttp.NetHTTP {
		clt := &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				Proxy: http.ProxyFromEnvironment,
				DialContext: (&net.Dialer{
					Timeout:   10 * time.Second,
					KeepAlive: time.Duration(keepAlivePeriod) * time.Second,
					DualStack: true,
				}).DialContext,
				MaxIdleConns:          maxIdleConns,
				MaxIdleConnsPerHost:   maxIdleConnsPerHost,
				IdleConnTimeout:       90 * time.Second,
				TLSHandshakeTimeout:   10 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
				TLSClientConfig:       tlsConfig,
			},
		}
		return &netHTTPClient{clt}
	}
	clt := &fasthttp.Client{
		// Idle keep-alive connections are closed after this duration.
		// By default idle connections are closed
		// after DefaultMaxIdleConnDuration.
		// fasthttp client cannot reliably detect server closes incoming connection
		// after KeepAliveTimeout. So MaxIdleConnDuration should be smaller than
		// server's KeepAliveTimeout, see https://github.com/valyala/fasthttp/issues/189
		MaxIdleConnDuration: time.Duration(keepAlivePeriod) * time.Second,
		DialDualStack:       true,
		TLSConfig:           tlsConfig,
		MaxConnsPerHost:     maxIdleConnsPerHost,
		// net package's DefaultDialTimeout is no timeout
		// fasthttp's DefaultDialTimeout is 3 second, so change it here
		Dial: func(addr string) (net.Conn, error) {
			return fasthttp.DialTimeout(addr, 10*time.Second)
		},
	}
	return &fastHTTPClient{clt, timeout}
}

////
func copyRequestHeaderFromTask(t task.Task, key string, dst plugins.Header) {
	if src, ok := t.Value(key).(plugins.Header); !ok {
		// There are some normal cases that the header key is nil in task
		// Because header key producer don't write them
		logger.Debugf("[load header: %s in the task failed, header is %+v]", key, t.Value(key))
	} else {
		if err := src.CopyTo(dst); err != nil {
			logger.Warnf("[copyRequestHeaderFromTask failed: %v]", err)
		}
	}
}
