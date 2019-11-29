package context

import (
	"io"
	"net/http"

	"github.com/megaease/easegateway/pkg/util/httpheader"
	"github.com/megaease/easegateway/pkg/util/readercounter"
	"github.com/megaease/easegateway/pkg/util/stringtool"
	"github.com/tomasen/realip"
)

type (
	httpRequest struct {
		std      *http.Request
		method   string
		path     string
		header   *httpheader.HTTPHeader
		body     *readercounter.ReaderCounter
		metaSize uint64
		bodySize uint64
		realIP   string
	}
)

func newHTTPRequest(stdr *http.Request) *httpRequest {
	hq := &httpRequest{
		std:    stdr,
		method: stdr.Method,
		path:   stdr.URL.Path,
		header: httpheader.New(stdr.Header),
		body:   readercounter.New(stdr.Body),
		realIP: realip.FromRequest(stdr),
	}

	// Reference: https://tools.ietf.org/html/rfc2616#section-5
	// NOTE: We don't use httputil.DumpRequest because it does not
	// completely output plain HTTP Request.
	meta := stringtool.Cat(stdr.Method, " ", stdr.URL.RequestURI(), " ", stdr.Proto, "\r\n",
		hq.Header().Dump(), "\r\n\r\n")

	hq.metaSize = uint64(len(meta))

	return hq
}

func (r *httpRequest) RealIP() string {
	return r.realIP
}

func (r *httpRequest) Method() string {
	return r.method
}

func (r *httpRequest) SetMethod(method string) {
	r.method = method
}

func (r *httpRequest) Scheme() string {
	return r.std.URL.Scheme
}

func (r *httpRequest) Host() string {
	return r.std.Host
}

func (r *httpRequest) Path() string {
	return r.path
}

func (r *httpRequest) SetPath(path string) {
	r.path = path
}

func (r *httpRequest) EscapedPath() string {
	return r.std.URL.EscapedPath()
}

func (r *httpRequest) Query() string {
	return r.std.URL.RawQuery
}

func (r *httpRequest) Fragment() string {
	return r.std.URL.Fragment
}

func (r *httpRequest) Proto() string {
	return r.std.Proto
}

func (r *httpRequest) Header() *httpheader.HTTPHeader {
	return r.header
}

func (r *httpRequest) Cookie(name string) (*http.Cookie, error) {
	return r.std.Cookie(name)
}

func (r *httpRequest) Cookies() []*http.Cookie {
	return r.std.Cookies()
}

func (r *httpRequest) AddCookie(cookie *http.Cookie) {
	r.std.AddCookie(cookie)
}

func (r *httpRequest) Body() io.Reader {
	return r.body
}

func (r *httpRequest) SetBody(reader io.Reader) {
	if r.bodySize == 0 {
		// NOTE: Always store original body size.
		r.bodySize = r.body.Count()
	}
	r.body = readercounter.New(reader)
}

func (r *httpRequest) Size() uint64 {
	if r.bodySize != 0 {
		// NOTE: Always load original body size.
		return r.metaSize + r.bodySize
	}
	return r.metaSize + r.body.Count()
}

func (r *httpRequest) finish() {
	// NOTE: We don't use this line in case of large flow attack.
	// io.Copy(ioutil.Discard, r.std.Body)

	r.std.Body.Close()
}
