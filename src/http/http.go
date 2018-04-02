package http

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"reflect"
	"strconv"
	"strings"

	"github.com/hexdecteam/easegateway-types/plugins"

	"github.com/erikdubbelboer/fasthttp"
	"github.com/erikdubbelboer/fasthttp/fasthttputil"

	"common"
	"logger"
)

//// This file implementes server side HTTP related interfaces defined in easegateway-types/plugins.go

// Indicates whether dump request/response body when dumping request/response
// it's risky to open this in production mode in case there are big streaming
// request/response
var DumpBody = false

const (
	NetHTTP plugins.HTTPType = iota
	FastHTTP
)

const (
	NetHTTPStr  = "nethttp"
	FastHTTPStr = "fasthttp"

	DefaultHTTPPort  = "80"
	DefaultHTTPSPort = "443"
)

var httpImpls = map[string]plugins.HTTPType{
	NetHTTPStr:  NetHTTP,
	FastHTTPStr: FastHTTP,
}

func ParseHTTPType(s string) (plugins.HTTPType, error) {
	if impl, ok := httpImpls[s]; ok {
		return impl, nil
	}
	return -1, fmt.Errorf("unsupported http implementation type: %s", s)
}

type VisitAllKV func(k, v string)

// FastRequestHeader leverages fasthttp, implements plugins.Header interface
type FastRequestHeader struct {
	isTLS  bool
	uri    *fasthttp.URI
	header *fasthttp.RequestHeader
}

func NewFastRequestHeader(isTLS bool, uri *fasthttp.URI, header *fasthttp.RequestHeader) plugins.Header {
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
	if matches := common.TrailingPort.FindStringSubmatch(host); len(matches) != 0 {
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

func (f *FastRequestHeader) CopyTo(dst plugins.Header) error {
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

// Add adds the given 'key: value' header.
// Multiple headers with the same key may be added with this function.
// Use Set for setting a single header for the given key.
func (f *FastRequestHeader) Add(k, v string) {
	f.header.Add(k, v)
}

func (f *FastRequestHeader) SetContentLength(len int64) {
	f.header.SetContentLength(int(len))
}

// NetRequestHeader leverages net/http, implements plugins.Header interface
type NetRequestHeader struct {
	R *http.Request
}

func NewNetRequestHeader(r *http.Request) plugins.Header {
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
	if matches := common.TrailingPort.FindStringSubmatch(f.R.URL.Host); len(matches) != 0 {
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

func (f *NetRequestHeader) CopyTo(dst plugins.Header) error {
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

// NetResponseHeader leverages net/http, implements plugins.Header interface
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

// Add adds the given 'key: value' header.
// Multiple headers with the same key may be added with this function.
// Use Set for setting a single header for the given key.
func (f *NetResponseHeader) Add(k, v string) {
	f.H.Add(k, v)
}

func (f *NetResponseHeader) CopyTo(dst plugins.Header) error {
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

func (f *FastResponseHeader) Add(k, v string) {
	f.H.Add(k, v)
}

func (f *FastResponseHeader) CopyTo(dst plugins.Header) error {
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

// FastResponseHeader leverages fasthttp, implements plugins.Header interface
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

// NetHTTPCtx leverages net/http, implements plugins.HTTPCtx
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

func (n *NetHTTPCtx) RequestHeader() plugins.Header {
	return &NetRequestHeader{R: n.Req}
}
func (n *NetHTTPCtx) ResponseHeader() plugins.Header {
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

func (f *NetHTTPCtx) BodyReadCloser() plugins.SizedReadCloser {
	reader := common.NewSizedReadCloser(f.Req.Body, f.Req.ContentLength)
	return reader
}

// FastHTTPCtx leverages fasthttp, implements plugins.HTTPCtx
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

func (f *FastHTTPCtx) BodyReadCloser() plugins.SizedReadCloser {
	// fasthttputil.NewPipeConns uses memory buffer
	// and is faster than io.Pipe()
	c := fasthttputil.NewPipeConns()
	r := c.Conn1()
	w := c.Conn2()
	go func() {
		_ = f.Req.BodyWriteTo(w) // ignore error safely
		w.Close()
	}()

	return common.NewSizedReadCloser(r, int64(f.Req.Header.ContentLength()))
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

func (n *FastHTTPCtx) RequestHeader() plugins.Header {
	return NewFastRequestHeader(n.Ctx.IsTLS(), n.Req.URI(), &n.Req.Header)
}

func (n *FastHTTPCtx) ResponseHeader() plugins.Header {
	return &FastResponseHeader{H: &n.Resp.Header}
}
