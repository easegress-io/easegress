package httpserver

import (
	"io"
	"net/http"

	"github.com/megaease/easegateway/pkg/util/httpheader"
	"github.com/megaease/easegateway/pkg/util/readercounter"
)

type (
	httpRequest struct {
		std    *http.Request
		header *httpheader.HTTPHeader
		body   *readercounter.ReaderCounter
		realIP string
	}
)

func (r *httpRequest) RealIP() string {
	return r.realIP
}

func (r *httpRequest) Method() string {
	return r.std.Method
}

func (r *httpRequest) Scheme() string {
	return r.std.URL.Scheme
}

func (r *httpRequest) Host() string {
	return r.std.Host
}

func (r *httpRequest) Path() string {
	return r.std.URL.Path
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

func (r *httpRequest) Size() uint64 {
	return r.body.Count()
}
