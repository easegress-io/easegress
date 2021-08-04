package proxy

import (
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/megaease/easegress/pkg/context/contexttest"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/util/httpheader"
)

func TestMain(m *testing.M) {
	logger.InitNop()
	code := m.Run()
	os.Exit(code)
}

func TestAcceptGzip(t *testing.T) {
	c := newCompression(&CompressionSpec{MinLength: 100})

	header := http.Header{}
	ctx := &contexttest.MockedHTTPContext{}
	ctx.MockedRequest.MockedHeader = func() *httpheader.HTTPHeader {
		return httpheader.New(header)
	}

	if !c.acceptGzip(ctx) {
		t.Error("accept gzip should be true")
	}

	header.Add(httpheader.KeyAcceptEncoding, "text/text")
	if c.acceptGzip(ctx) {
		t.Error("accept gzip should be false")
	}

	header.Add(httpheader.KeyAcceptEncoding, "*/*")
	if !c.acceptGzip(ctx) {
		t.Error("accept gzip should be true")
	}

	header.Del(httpheader.KeyAcceptEncoding)
	header.Add(httpheader.KeyAcceptEncoding, "gzip")
	if !c.acceptGzip(ctx) {
		t.Error("accept gzip should be true")
	}
}

func TestAlreadyGziped(t *testing.T) {
	c := newCompression(&CompressionSpec{MinLength: 100})

	header := http.Header{}
	ctx := &contexttest.MockedHTTPContext{}
	ctx.MockedResponse.MockedHeader = func() *httpheader.HTTPHeader {
		return httpheader.New(header)
	}

	if c.alreadyGziped(ctx) {
		t.Error("already gziped should be false")
	}

	header.Add(httpheader.KeyContentEncoding, "text")
	if c.alreadyGziped(ctx) {
		t.Error("already gziped should be false")
	}

	header.Add(httpheader.KeyContentEncoding, "gzip")
	if !c.alreadyGziped(ctx) {
		t.Error("already gziped should be true")
	}
}

func TestParseContentLength(t *testing.T) {
	c := newCompression(&CompressionSpec{MinLength: 100})

	header := http.Header{}
	ctx := &contexttest.MockedHTTPContext{}
	ctx.MockedResponse.MockedHeader = func() *httpheader.HTTPHeader {
		return httpheader.New(header)
	}

	if c.parseContentLength(ctx) != -1 {
		t.Error("content length should be -1")
	}

	header.Set(httpheader.KeyContentLength, "abc")
	if c.parseContentLength(ctx) != -1 {
		t.Error("content length should be -1")
	}

	header.Set(httpheader.KeyContentLength, "100")
	if c.parseContentLength(ctx) != 100 {
		t.Error("content length should be 100")
	}
}

func TestCompress(t *testing.T) {
	c := newCompression(&CompressionSpec{MinLength: 100})

	header := http.Header{}
	ctx := &contexttest.MockedHTTPContext{}
	ctx.MockedRequest.MockedHeader = func() *httpheader.HTTPHeader {
		return httpheader.New(header)
	}
	ctx.MockedResponse.MockedHeader = func() *httpheader.HTTPHeader {
		return httpheader.New(header)
	}
	header.Set(httpheader.KeyContentLength, "20")

	rawBody := strings.Repeat("this is the raw body. ", 100)
	sr := strings.NewReader(rawBody)
	ctx.MockedResponse.MockedBody = func() io.Reader {
		return sr
	}

	c.compress(ctx)
	if header.Get(httpheader.KeyContentEncoding) == "gzip" {
		t.Error("body should not be gziped")
	}

	ctx.MockedResponse.MockedSetBody = func(body io.Reader) {
		io.ReadAll(body)
	}

	header.Set(httpheader.KeyContentLength, "120")
	c.compress(ctx)
	if header.Get(httpheader.KeyContentEncoding) != "gzip" {
		t.Error("body should be gziped")
	}

}
