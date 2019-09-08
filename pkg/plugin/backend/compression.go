package backend

import (
	"bytes"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/megaease/easegateway/pkg/context"
	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/util/httpheader"

	"github.com/klauspost/compress/gzip"
)

// TODO: Expose more options: compression level, mime types.

var (
	bodyFlushSize = 8 * int64(os.Getpagesize())
)

type (
	gzipBody struct {
		body     io.Reader
		buff     *bytes.Buffer
		gw       *gzip.Writer
		complete bool
	}

	// compression is plugin compression.
	compression struct {
		spec *CompressionSpec
	}

	// CompressionSpec describes the compression.
	CompressionSpec struct {
		MinLength uint32 `yaml:"minLength"`
	}
)

func newcompression(spec *CompressionSpec) *compression {
	return &compression{
		spec: spec,
	}
}

func (c *compression) compress(ctx context.HTTPContext) {
	if !c.acceptGzip(ctx) {
		return
	}

	if c.alreadyGziped(ctx) {
		return
	}

	cl := c.parseContentLength(ctx)
	if cl != -1 && cl < int(c.spec.MinLength) {
		return
	}
	w := ctx.Response()
	if w.Body() == nil {
		return
	}

	ctx.Response().Header().Del(httpheader.KeyContentLength)

	w.Header().Set(httpheader.KeyContentEncoding, "gzip")
	w.Header().Add(httpheader.KeyVary, httpheader.KeyContentEncoding)

	ctx.AddTag("gzip")

	w.SetBody(newGzipBody(w.Body()))
}

func (c *compression) alreadyGziped(ctx context.HTTPContext) bool {
	for _, ce := range ctx.Response().Header().GetAll(httpheader.KeyContentEncoding) {
		if strings.Contains(ce, "gzip") {
			return true
		}
	}

	return false
}

func (c *compression) acceptGzip(ctx context.HTTPContext) bool {
	acceptEncodings := ctx.Request().Header().GetAll(httpheader.KeyAcceptEncoding)

	// NOTE: EaseGateway does not support parsing qvalue for performace.
	// Reference: https://tools.ietf.org/html/rfc2616#section-14.3
	if len(acceptEncodings) > 0 {
		for _, ae := range acceptEncodings {
			if strings.Contains(ae, "*/*") ||
				strings.Contains(ae, "gzip") {
				return true
			}
		}
		return false
	}

	return true
}

func (c *compression) parseContentLength(ctx context.HTTPContext) int {
	contentLength := ctx.Response().Header().Get(httpheader.KeyContentLength)
	if contentLength == "" {
		return -1
	}

	cl, err := strconv.ParseInt(contentLength, 10, 64)
	if err != nil {
		return -1
	}

	return int(cl)
}

func newGzipBody(body io.Reader) *gzipBody {
	buff := bytes.NewBuffer(nil)
	return &gzipBody{
		body: body,
		buff: buff,
		gw:   gzip.NewWriter(buff),
	}
}

// body -> gw -> p
func (gb *gzipBody) Read(p []byte) (int, error) {
	if gb.complete {
		return 0, io.EOF
	}

	if len(gb.buff.Bytes()) < len(p) {
		gb.pull()
	}

	n, err := gb.buff.Read(p)
	if err == io.EOF && !gb.complete {
		err = nil
	}

	return n, err
}

func (gb *gzipBody) pull() {
	_, err := io.CopyN(gb.gw, gb.body, bodyFlushSize)
	switch err {
	case nil:
		// Nothing to do.
	case io.EOF:
		err := gb.gw.Close()
		if err != nil {
			logger.Errorf("BUG: close gzip failed: %v", err)
		}
		gb.complete = true
	default:
		gb.complete = true
		logger.Errorf("BUG: copy body to gzip failed: %v", err)
	}
}
