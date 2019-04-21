package httpserver

import (
	"bytes"
	"io"
	"net/http"
	"os"

	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/util/httpheader"
)

var (
	bodyFlushBuffSize = int64(os.Getpagesize())
)

type (
	// BodyFlushFunc is the type of function to be called back
	// when body is flushing.
	BodyFlushFunc = func(body []byte, complete bool) (newBody []byte)

	httpResponse struct {
		std http.ResponseWriter

		code   int
		header *httpheader.HTTPHeader

		body           io.Reader
		bodyWritten    int64
		bodyFlushFuncs []BodyFlushFunc
	}
)

func (w *httpResponse) StatusCode() int {
	return w.code
}

func (w *httpResponse) SetStatusCode(code int) {
	w.code = code
}

func (w *httpResponse) Header() *httpheader.HTTPHeader {
	return w.header
}

func (w *httpResponse) Body() io.Reader {
	return w.body
}

func (w *httpResponse) SetBody(body io.Reader) {
	w.body = body
}

// None uses it currently, keep it for future maybe.
func (w *httpResponse) OnFlushBody(fn BodyFlushFunc) {
	w.bodyFlushFuncs = append(w.bodyFlushFuncs, fn)
}

func (w *httpResponse) flushBody() {
	if w.body == nil {
		return
	}

	defer func() {
		if body, ok := w.body.(io.ReadCloser); ok {
			// NOTICE: Need to be read to completion and closed.
			// Reference: https://golang.org/pkg/net/http/#Response
			err := body.Close()
			if err != nil {
				logger.Warnf("close body failed: %v", err)
			}
		}
	}()

	copyToClient := func(src io.Reader) (succeed bool) {
		written, err := io.Copy(w.std, src)
		if err != nil {
			logger.Warnf("copy body failed: %v", err)
			return false
		}
		w.bodyWritten += written
		return true
	}

	if len(w.bodyFlushFuncs) == 0 {
		copyToClient(w.body)
		return
	}

	buff := bytes.NewBuffer(nil)
	for {
		buff.Reset()
		_, err := io.CopyN(buff, w.body, bodyFlushBuffSize)
		body := buff.Bytes()

		switch err {
		case nil:
			// Switch to chunked mode (EaseGateway defined).
			// Reference: https://gist.github.com/CMCDragonkai/6bfade6431e9ffb7fe88
			// NOTICE: Golang server will adjust it according to the content length.
			// if !chunkedMode {
			// 	chunkedMode = true
			// 	w.Header().Del("Content-Length")
			// 	w.Header().Set("Transfer-Encoding", "chunked")
			// }

			for _, fn := range w.bodyFlushFuncs {
				body = fn(body, false /*not complete*/)
			}
			if !copyToClient(bytes.NewReader(body)) {
				return
			}
		case io.EOF:
			for _, fn := range w.bodyFlushFuncs {
				body = fn(body, true /*complete*/)
			}

			copyToClient(bytes.NewReader(body))
			return
		default:
			w.SetStatusCode(http.StatusInternalServerError)
			return
		}
	}
}

func (w *httpResponse) FlushedBodyBytes() int64 {
	return w.bodyWritten
}

func (w *httpResponse) finish() {
	// NOTICE: WriteHeader must be called at most one time.
	w.std.WriteHeader(w.StatusCode())
	w.flushBody()
}
