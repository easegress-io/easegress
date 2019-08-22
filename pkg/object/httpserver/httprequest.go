package httpserver

import (
	"io"
	"net/http"

	"github.com/megaease/easegateway/pkg/util/httpheader"
	"github.com/megaease/easegateway/pkg/util/readercounter"
)

type (
	httpRequest struct {
		std      *http.Request
		method   string
		path     string
		header   *httpheader.HTTPHeader
		body     *readercounter.ReaderCounter
		bodySize uint64
		realIP   string
	}
)

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
		return r.bodySize
	}
	return r.body.Count()
}

func (r *httpRequest) finish() {
	// NOTE: We don't use this line in case of large flow attack.
	// io.Copy(ioutil.Discard, r.std.Body)

	r.std.Body.Close()
}
