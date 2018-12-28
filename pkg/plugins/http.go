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
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/megaease/easegateway/pkg/common"
	"github.com/megaease/easegateway/pkg/logger"

	"github.com/erikdubbelboer/fasthttp"
	"github.com/erikdubbelboer/fasthttp/fasthttputil"

	"github.com/megaease/easegateway/pkg/task"
)

//// This file implementes server side HTTP related interfaces defined in easegateway-types/go

// Indicates whether dump request/response body when dumping request/response
// it's risky to open this in production mode in case there are big streaming
// request/response
var DumpBody = false

const (
	NetHTTP HTTPType = iota
	FastHTTP
)

const (
	NetHTTPStr  = "nethttp"
	FastHTTPStr = "fasthttp"

	DefaultHTTPPort  = "80"
	DefaultHTTPSPort = "443"
)

var httpImpls = map[string]HTTPType{
	NetHTTPStr:  NetHTTP,
	FastHTTPStr: FastHTTP,
}

func ParseHTTPType(s string) (HTTPType, error) {
	if impl, ok := httpImpls[s]; ok {
		return impl, nil
	}
	return -1, fmt.Errorf("unsupported http implementation type: %s", s)
}

type VisitAllKV func(k, v string)

// FastRequestHeader leverages fasthttp, implements Header interface
type FastRequestHeader struct {
	isTLS  bool
	uri    *fasthttp.URI
	header *fasthttp.RequestHeader
}

func NewFastRequestHeader(isTLS bool, uri *fasthttp.URI, header *fasthttp.RequestHeader) Header {
	return &FastRequestHeader{
		isTLS:  isTLS,
		uri:    uri,
		header: header,
	}
}

func (f *FastRequestHeader) Proto() string {
	// fasthttp doesn't support HTTP/2
	if f.header.IsHTTP11() {
		return "HTTP/1.1"
	}
	return "HTTP/1.0"
}

func (f *FastRequestHeader) Method() string {
	return common.B2s(f.header.Method())
}

func (f *FastRequestHeader) Get(k string) string {
	return common.B2s(f.header.Peek(k))
}

func (f *FastRequestHeader) Host() string {
	host := common.B2s(f.header.Host())
	if !strings.Contains(host, ":") {
		return host + ":80"
	}
	return host
}

func (f *FastRequestHeader) Scheme() string {
	if f.isTLS {
		return "https"
	}
	return "http"
}

func (f *FastRequestHeader) Path() string {
	return common.B2s(f.uri.Path())
}

// full url, for example: http://www.google.com:80/search?s=megaease#title
func (f *FastRequestHeader) FullURI() string {
	// URI.Host contains port
	host := common.B2s(f.uri.Host())
	if matches := TrailingPort.FindStringSubmatch(host); len(matches) != 0 {
		return fmt.Sprintf("%s://%s%s?%s#%s", f.Scheme(), host, common.B2s(f.uri.Path()), common.B2s(f.uri.QueryString()), common.B2s(f.uri.Hash()))
	}
	port := DefaultHTTPPort
	if f.Scheme() == "https" {
		port = DefaultHTTPSPort
	}
	return fmt.Sprintf("%s://%s:%s%s?%s#%s", f.Scheme(), host, port, common.B2s(f.uri.Path()), common.B2s(f.uri.QueryString()), common.B2s(f.uri.Hash()))
}

func (f *FastRequestHeader) QueryString() string {
	return common.B2s(f.uri.QueryString())
}

func (f *FastRequestHeader) ContentLength() int64 {
	return int64(f.header.ContentLength())
}

// VisitAll calls f for each header.
func (f *FastRequestHeader) VisitAll(fun func(k, v string)) {
	wrapFunc := func(k, v []byte) {
		fun(common.B2s(k), common.B2s(v))
	}
	f.header.VisitAll(wrapFunc)
}

func (f *FastRequestHeader) CopyTo(dst Header) error {
	switch value := dst.(type) {
	case *NetRequestHeader:
		copy := func(k, v []byte) {
			// EaseGateway is performing as http proxy
			// So overrite existing "User-Agent"
			if common.B2s(k) == "User-Agent" {
				value.Set(common.B2s(k), common.B2s(v))
			} else {
				value.Add(common.B2s(k), common.B2s(v))
			}
		}
		f.header.VisitAll(copy)
	case *FastRequestHeader:
		method := string(value.Method())
		url := string(value.FullURI())
		f.header.CopyTo(value.header)
		// recover method, url
		value.header.SetMethod(method)
		value.header.SetRequestURI(url)
	default:
		return fmt.Errorf("unkown dst type: %v, value: %+v", reflect.TypeOf(dst), dst)
	}
	return nil
}

// Set sets the given 'key: value' header.
func (f *FastRequestHeader) Set(k, v string) {
	f.header.Set(k, v)
}

func (f *FastRequestHeader) Del(k string) {
	f.header.Del(k)
}

// Add adds the given 'key: value' header.
// Multiple headers with the same key may be added with this function.
// Use Set for setting a single header for the given key.
func (f *FastRequestHeader) Add(k, v string) {
	f.header.Add(k, v)
}

func (f *FastRequestHeader) SetContentLength(len int64) {
	f.header.SetContentLength(int(len))
}

// NetRequestHeader leverages net/http, implements Header interface
type NetRequestHeader struct {
	R *http.Request
}

func NewNetRequestHeader(r *http.Request) Header {
	return &NetRequestHeader{
		R: r,
	}
}
func (f *NetRequestHeader) Proto() string {
	return f.R.Proto
}

func (f *NetRequestHeader) Method() string {
	return f.R.Method
}

func (f *NetRequestHeader) Get(k string) string {
	return strings.Join(f.R.Header[k], ", ")
}

func (f *NetRequestHeader) Host() string {
	return f.R.Host
}

func (f *NetRequestHeader) Scheme() string {
	if f.R.TLS != nil {
		return "https"
	}
	return "http"
}

func (f *NetRequestHeader) Path() string {
	return f.R.URL.Path
}

// full url, for example: http://www.google.com?s=megaease#title
func (f *NetRequestHeader) FullURI() string {
	// URL.Host contains port
	path := common.RemoveRepeatedByte(f.R.URL.Path, '/')
	if matches := TrailingPort.FindStringSubmatch(f.R.URL.Host); len(matches) != 0 {
		return fmt.Sprintf("%s://%s%s?%s#%s", f.Scheme(), f.R.URL.Host, path, f.R.URL.RawQuery, f.R.URL.Fragment)
	}

	port := DefaultHTTPPort
	if f.Scheme() == "https" {
		port = DefaultHTTPSPort
	}
	return fmt.Sprintf("%s://%s:%s%s?%s#%s", f.Scheme(), f.R.URL.Host, port, path, f.R.URL.RawQuery, f.R.URL.Fragment)
}

func (f *NetRequestHeader) QueryString() string {
	return f.R.URL.RawQuery
}

func (f *NetRequestHeader) ContentLength() int64 {
	return f.R.ContentLength
}

func (f *NetRequestHeader) ContentType() string {
	return f.R.Header.Get("Content-Type")
}

// VisitAll calls f for each header.
func (f *NetRequestHeader) VisitAll(fun func(k, v string)) {
	for k, v := range f.R.Header {
		joinStr := ", "
		if k == "COOKIE" {
			joinStr = "; "
		}
		fun(k, strings.Join(v, joinStr))
	}
}

func (f *NetRequestHeader) CopyTo(dst Header) error {
	var copy VisitAllKV
	var retErr error
	switch value := dst.(type) {
	case *NetRequestHeader:
		copy = func(k, v string) {
			// EaseGateway is performing as http proxy
			// So overwrite existing "User-Agent"
			switch k {
			case "User-Agent":
				value.Set(k, v)

			default:
				value.Add(k, v)
			}
		}
	case *FastRequestHeader:
		copy = func(k, v string) {
			switch k {
			case "Content-Type":
				value.header.SetContentType(v)
			case "Content-Length":
				if len, err := strconv.ParseInt(v, 10, 64); err == nil {
					dst.SetContentLength(len)
				} else {
					retErr = fmt.Errorf("parse Content-Length header %s to int failed: %v", v, err)
				}
			case "User-Agent":
				dst.Set(k, v)
			default:
				dst.Add(k, v)
			}
		}
	default:
		return fmt.Errorf("unkown dst type: %v, value: %+v", reflect.TypeOf(dst), dst)
	}
	for k, v := range f.R.Header {
		for _, vv := range v {
			copy(k, vv)
		}
	}
	return retErr
}

// Set sets the given 'key: value' header.
func (f *NetRequestHeader) Set(k, v string) {
	if k == "Host" { // https://github.com/golang/go/issues/7682
		f.R.Host = v
	} else {
		f.R.Header.Set(k, v)
	}
}

// Del deletes header with the given key
func (f *NetRequestHeader) Del(k string) {
	f.R.Header.Del(k)
}

// Add adds the given 'key: value' header.
// Multiple headers with the same key may be added with this function.
// Use Set for setting a single header for the given key.
func (f *NetRequestHeader) Add(k, v string) {
	f.R.Header.Add(k, v)
}

func (f *NetRequestHeader) SetContentLength(len int64) {
	f.R.ContentLength = len
}

//////// server side response header

// NetResponseHeader leverages net/http, implements Header interface
type NetResponseHeader struct {
	H http.Header
}

func (f *NetResponseHeader) Get(k string) string {
	return f.H.Get(k)
}

func (f *NetResponseHeader) ContentLength() int64 {
	v := f.H.Get("Content-Length")
	if len, err := strconv.ParseInt(v, 10, 64); err == nil {
		return len
	}
	// unkown length
	return -1
}

func (f *NetResponseHeader) VisitAll(fun func(k, v string)) {
	for k, v := range f.H {
		for _, vv := range v {
			fun(k, vv)
		}
	}
}

// Set sets the given 'key: value' header.
func (f *NetResponseHeader) Set(k, v string) {
	f.H.Set(k, v)
}

// Del deletes header with the given key
func (f *NetResponseHeader) Del(k string) {
	f.H.Del(k)
}

// Add adds the given 'key: value' header.
// Multiple headers with the same key may be added with this function.
// Use Set for setting a single header for the given key.
func (f *NetResponseHeader) Add(k, v string) {
	f.H.Add(k, v)
}

func (f *NetResponseHeader) CopyTo(dst Header) error {
	var copy VisitAllKV
	switch value := dst.(type) {
	case *NetResponseHeader:
		copy = func(k, v string) {
			value.Add(k, v)
		}
	case *FastResponseHeader:
		copy = func(k, v string) {
			switch k {
			case "Content-Type":
				value.Set("Content-Type", v)
			case "Content-Length":
				if len, err := strconv.ParseInt(v, 10, 64); err == nil {
					dst.SetContentLength(len)
				} else {
					// maybe move this files into http package?
					// Warnf("parse Content-Length header %s to int failed: %v", v, err)
				}
			default:
				dst.Add(k, v)
			}
		}
	default:
		return fmt.Errorf("unkown dst type: %v, value: %+v", reflect.TypeOf(dst), dst)
	}
	for k, v := range f.H {
		for _, vv := range v {
			copy(k, vv)
		}
	}
	return nil
}

func (f *NetResponseHeader) SetContentLength(len int64) {
	f.H.Set("Content-Length", strconv.FormatInt(len, 10))
}
func (f *FastResponseHeader) ContentLength() int64 {
	return int64(f.H.ContentLength())
}

func (f *FastResponseHeader) VisitAll(fun func(k, v string)) {
	wrapFun := func(k, v []byte) {
		fun(common.B2s(k), common.B2s(v))
	}
	f.H.VisitAll(wrapFun)
}

func (f *FastResponseHeader) Set(k, v string) {
	f.H.Set(k, v)
}

func (f *FastResponseHeader) Del(k string) {
	f.H.Del(k)
}

func (f *FastResponseHeader) Add(k, v string) {
	f.H.Add(k, v)
}

func (f *FastResponseHeader) CopyTo(dst Header) error {
	switch value := dst.(type) {
	case *NetResponseHeader:
		copy := func(k, v []byte) {
			value.Add(common.B2s(k), common.B2s(v))
		}
		f.H.VisitAll(copy)
	case *FastResponseHeader:
		f.H.CopyTo(value.H)
	default:
		return fmt.Errorf("unkown dst type: %v, value: %+v", reflect.TypeOf(dst), dst)
	}
	return nil
}

func (f *FastResponseHeader) SetContentLength(len int64) {
	f.H.SetContentLength(int(len))
}

// These below methods are currently not supported on NetResponseHeader because
// current implementation don't need them
func (f *NetResponseHeader) Proto() string {
	logger.Warnf("[BUG: don't support this method on server side response header")
	return ""
}

func (f *NetResponseHeader) Method() string {
	logger.Warnf("[BUG: don't support this method on server side response header")
	return ""
}

func (f *NetResponseHeader) Host() string {
	logger.Warnf("[BUG: don't support this method on server side response header")
	return ""
}

func (f *NetResponseHeader) Scheme() string {
	logger.Warnf("[BUG: don't support this method on server side response header")
	return ""
}

func (f *NetResponseHeader) Path() string {
	logger.Warnf("[BUG: don't support this method on server side response header")
	return ""
}

// full url, for example: http://www.google.com?s=megaease#title
func (f *NetResponseHeader) FullURI() string {
	logger.Warnf("[BUG: don't support this method on server side response header")
	return ""
}

func (f *NetResponseHeader) QueryString() string {
	logger.Warnf("[BUG: don't support this method on server side response header")
	return ""
}

// FastResponseHeader leverages fasthttp, implements Header interface
type FastResponseHeader struct {
	H *fasthttp.ResponseHeader
}

func (f *FastResponseHeader) Get(k string) string {
	return common.B2s(f.H.Peek(k))
}

// These below methods are currently not supported on FastResponseHeader because
// current implementation don't need them
func (f *FastResponseHeader) Proto() string {
	logger.Warnf("[BUG: don't support this method on server side response header")
	return ""
}

func (f *FastResponseHeader) Method() string {
	logger.Warnf("[BUG: don't support this method on server side response header")
	return ""
}

func (f *FastResponseHeader) Host() string {
	logger.Warnf("[BUG: don't support this method on server side response header")
	return ""
}

func (f *FastResponseHeader) Scheme() string {
	logger.Warnf("[BUG: don't support this method on server side response header")
	return ""
}

func (f *FastResponseHeader) Path() string {
	logger.Warnf("[BUG: don't support this method on server side response header")
	return ""
}

// full url, for example: http://www.google.com?s=megaease#title
func (f *FastResponseHeader) FullURI() string {
	logger.Warnf("[BUG: don't support this method on server side response header")
	return ""
}

func (f *FastResponseHeader) QueryString() string {
	logger.Warnf("[BUG: don't support this method on server side response header")
	return ""
}

// NetHTTPCtx leverages net/http, implements HTTPCtx
type NetHTTPCtx struct {
	Req    *http.Request
	Writer http.ResponseWriter
}

func (n *NetHTTPCtx) RemoteAddr() string {
	return n.Req.RemoteAddr
}

func (n *NetHTTPCtx) SetStatusCode(s int) {
	n.Writer.WriteHeader(s)
}

func (n *NetHTTPCtx) SetContentLength(len int64) {
	n.Writer.Header().Set("Content-Length", strconv.FormatInt(len, 10))
}

func (n *NetHTTPCtx) Write(p []byte) (int, error) {
	return n.Writer.Write(p)
}

func (n *NetHTTPCtx) RequestHeader() Header {
	return &NetRequestHeader{R: n.Req}
}
func (n *NetHTTPCtx) ResponseHeader() Header {
	return &NetResponseHeader{H: n.Writer.Header()}
}

func (n *NetHTTPCtx) DumpRequest() (string, error) {
	if s, e := httputil.DumpRequest(n.Req, DumpBody); e != nil {
		return "", e
	} else {
		return common.B2s(s), nil
	}
}

func (n *NetHTTPCtx) CloseNotifier() http.CloseNotifier {
	return n.Writer.(http.CloseNotifier)
}

func (f *NetHTTPCtx) BodyReadCloser() SizedReadCloser {
	reader := NewSizedReadCloser(f.Req.Body, f.Req.ContentLength)
	return reader
}

// FastHTTPCtx leverages fasthttp, implements HTTPCtx
type FastHTTPCtx struct {
	Resp *fasthttp.Response // use inner field to expose the http.ResponseWriter interface
	Req  *fasthttp.Request
	Ctx  *fasthttp.RequestCtx
}

// fasthttp don't support CloseNotifier
func (f *FastHTTPCtx) CloseNotifier() http.CloseNotifier {
	return nil
}

func (f *FastHTTPCtx) DumpRequest() (string, error) {
	r := fasthttp.AcquireRequest()
	// CopyTo copies req contents to dst except of body stream.
	// So this operation avoids drain request body stream
	f.Req.CopyTo(r)
	if DumpBody {
		body := f.Req.Body()
		r.SetBody(body)
	}

	s := r.String()

	fasthttp.ReleaseRequest(r)
	return s, nil
}

func (f *FastHTTPCtx) BodyReadCloser() SizedReadCloser {
	// fasthttputil.NewPipeConns uses memory buffer
	// and is faster than io.Pipe()
	c := fasthttputil.NewPipeConns()
	r := c.Conn1()
	w := c.Conn2()
	go func() {
		_ = f.Req.BodyWriteTo(w) // ignore error safely
		w.Close()
	}()

	return NewSizedReadCloser(r, int64(f.Req.Header.ContentLength()))
}

func (n *FastHTTPCtx) RemoteAddr() string {
	return n.Ctx.RemoteAddr().String()
}

func (n *FastHTTPCtx) SetStatusCode(s int) {
	n.Resp.SetStatusCode(s)
}

func (n *FastHTTPCtx) SetContentLength(len int64) {
	n.Resp.Header.SetContentLength(int(len))
}

func (n *FastHTTPCtx) Write(p []byte) (int, error) {
	return n.Resp.BodyWriter().Write(p)
}

func (n *FastHTTPCtx) RequestHeader() Header {
	return NewFastRequestHeader(n.Ctx.IsTLS(), n.Req.URI(), &n.Req.Header)
}

func (n *FastHTTPCtx) ResponseHeader() Header {
	return &FastResponseHeader{H: &n.Resp.Header}
}

//// This file defines and implementes client side HTTP related interfaces

//// client side response interface, only supports get methods
type response interface {
	BodyReadCloser() SizedReadCloser
	Header() Header
	StatusCode() int
	Dump() (string, error)
}

// netHTTPResponse leverages net/http, implements response interface
type netHTTPResponse struct {
	r *http.Response
}

func (n *netHTTPResponse) BodyReadCloser() SizedReadCloser {
	return NewSizedReadCloser(n.r.Body, n.r.ContentLength)
}

func (n *netHTTPResponse) Header() Header {
	return &NetResponseHeader{H: n.r.Header}
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

func (f *fastHTTPResponse) BodyReadCloser() SizedReadCloser {
	r, w := io.Pipe()
	go func() {
		err := f.r.BodyWriteTo(w)
		w.CloseWithError(err)
		fasthttp.ReleaseResponse(f.r)
	}()
	return NewSizedReadCloser(r, int64(f.r.Header.ContentLength()))
}

func (f *fastHTTPResponse) Header() Header {
	return &FastResponseHeader{
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
	Header() Header
	Do(task.Task) (response, time.Duration, error)
	Dump() (string, error)
}

// httpRequest leverages net/http, implements request interface
type httpRequest struct {
	req    *http.Request
	client *http.Client
}

func (h *httpRequest) Header() Header {
	return &NetRequestHeader{
		R: h.req,
	}
}

func (h *httpRequest) Dump() (string, error) {
	if s, e := httputil.DumpRequest(h.req, DumpBody); e != nil {
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

func (f *fastRequest) Header() Header {
	// currently it's ok to set isTLS to false here, because it's client side request
	// maybe set according to request url in the future
	return NewFastRequestHeader(false /* isTLS */, f.req.URI(), &f.req.Header)
}

func (h *fastRequest) Dump() (string, error) {
	r := fasthttp.AcquireRequest()
	// CopyTo copies req contents to dst except of body stream.
	// So this operation avoids drain request body stream
	h.req.CopyTo(r)
	if DumpBody {
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
	CreateRequest(method, url string, bodyReader SizedReadCloser) (request, error)
}

// netHTTPClient leverages net/http, implements client interface
type netHTTPClient struct {
	c *http.Client
}

func (n *netHTTPClient) CreateRequest(method, url string, bodyReader SizedReadCloser) (request, error) {
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

func (n *fastHTTPClient) CreateRequest(method, url string, bodyReader SizedReadCloser) (request, error) {
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
	if c.typ == NetHTTP {
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
func copyRequestHeaderFromTask(t task.Task, key string, dst Header) {
	if src, ok := t.Value(key).(Header); !ok {
		// There are some normal cases that the header key is nil in task
		// Because header key producer don't write them
		logger.Debugf("[load header: %s in the task failed, header is %+v]", key, t.Value(key))
	} else {
		if err := src.CopyTo(dst); err != nil {
			logger.Warnf("[copyRequestHeaderFromTask failed: %v]", err)
		}
	}
}
