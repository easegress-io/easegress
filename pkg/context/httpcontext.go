package context

import (
	stdcontext "context"
	"io"
	"time"

	"github.com/megaease/easegateway/pkg/util/httpheader"
)

type (
	// HTTPContext is all context of an HTTP processing.
	HTTPContext interface {
		Request() HTTPRequest
		Response() HTTPReponse

		stdcontext.Context
		Cancel(err error)
		Cancelled() bool

		Duration() time.Duration // For log, sample, etc.
		OnFinish(func())         // For setting final client statistics, etc.
		AddTag(tag string)       // For debug, log, etc.

		Log() string
	}

	// HTTPRequest is all operations for HTTP request.
	HTTPRequest interface {
		RealIP() string

		Method() string

		// URL
		Scheme() string
		Host() string
		Path() string
		EscapedPath() string
		Query() string
		Fragment() string

		Proto() string

		Header() *httpheader.HTTPHeader

		Body() io.Reader

		Size() uint64 // bytes
	}

	// HTTPReponse is all operations for HTTP response.
	HTTPReponse interface {
		StatusCode() int // Default is 200
		SetStatusCode(code int)

		Header() *httpheader.HTTPHeader

		SetBody(body io.Reader)
		Body() io.Reader
		OnFlushBody(func(body []byte, complete bool) (newBody []byte))

		Size() uint64 // bytes
	}
)
