package remoteplugin

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/megaease/easegateway/pkg/context"
	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/object/httppipeline"
)

const (
	// Kind is the kind of RemotePlugin.
	Kind = "RemotePlugin"

	resultFailed = "failed"

	// 64KB
	maxBobyBytes = 64 * 1024
	// 192KB
	maxContextBytes = 3 * maxBobyBytes
)

func init() {
	httppipeline.Register(&httppipeline.PluginRecord{
		Kind:            Kind,
		DefaultSpecFunc: DefaultSpec,
		NewFunc:         New,
		Results:         []string{resultFailed},
	})
}

var (
	// All RemotePlugin instances use one globalClient in order to reuse
	// some resounces such as keepalive connections.
	globalClient = &http.Client{
		// NOTE: Timeout could be no limit, real client or server could cancel it.
		Timeout: 0,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 60 * time.Second,
				DualStack: true,
			}).DialContext,
			TLSClientConfig: &tls.Config{
				// NOTE: Could make it an paramenter,
				// when the requests need cross WAN.
				InsecureSkipVerify: true,
			},
			DisableCompression: false,
			// NOTE: The large number of Idle Connctions can
			// reduce overhead of building connections.
			MaxIdleConns:          10240,
			MaxIdleConnsPerHost:   512,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}
)

// DefaultSpec returns default spec.
func DefaultSpec() *Spec {
	return &Spec{}

}

type (
	// RemotePlugin is the plugin making remote service acting like internal plugin.
	RemotePlugin struct {
		spec *Spec
	}

	// Spec describes RemotePlugin.
	Spec struct {
		httppipeline.PluginMeta `yaml:",inline"`

		URL string `yaml:"url" v:"required,uri"`
	}

	contextEntity struct {
		Request  requestEntity  `json:"request"`
		Response responseEntity `json:"response"`
	}

	requestEntity struct {
		RealIP string `json:"realIP"`

		Method string `json:"method"`

		Scheme   string `json:"scheme"`
		Host     string `json:"host"`
		Path     string `json:"path"`
		Query    string `json:"query"`
		Fragment string `json:"fragment"`

		Proto string `json:"proto"`

		Header http.Header `json:"header"`

		Body []byte `json:"body"`
	}

	responseEntity struct {
		StatusCode int         `json:"statusCode"`
		Header     http.Header `json:"header"`
		Body       []byte      `json:"body"`
	}
)

// New creates a RemotePlugin.
func New(spec *Spec, prev *RemotePlugin) *RemotePlugin {
	return &RemotePlugin{
		spec: spec,
	}
}

func (rp *RemotePlugin) limitRead(reader io.Reader, n int64) []byte {
	if reader == nil {
		return nil
	}

	buff := bytes.NewBuffer(nil)
	written, err := io.CopyN(buff, reader, n+1)
	if err == nil && written == n+1 {
		panic(fmt.Errorf("larger than %dB", n))
	}

	if err != nil && err != io.EOF {
		panic(err)
	}

	return buff.Bytes()
}

// Handle handles HTTPContext by calling remote service.
func (rp *RemotePlugin) Handle(ctx context.HTTPContext) (result string) {
	r, w := ctx.Request(), ctx.Response()

	var errPrefix string
	defer func() {
		if err := recover(); err != nil {
			w.SetStatusCode(http.StatusServiceUnavailable)
			ctx.AddTag(fmt.Sprintf("remotePluginErr: %s: %v", errPrefix, err))
			result = resultFailed
		}
	}()

	errPrefix = "read request body"
	reqBody := rp.limitRead(r.Body(), maxBobyBytes)

	errPrefix = "read response body"
	respBody := rp.limitRead(w.Body(), maxBobyBytes)

	errPrefix = "marshal context"
	ctxBuff := rp.marshalHTTPContext(ctx, reqBody, respBody)

	req, err := http.NewRequest(http.MethodPost, rp.spec.URL, bytes.NewReader(ctxBuff))
	if err != nil {
		logger.Errorf("BUG: new request failed: %v", err)
		w.SetStatusCode(http.StatusInternalServerError)
		ctx.AddTag(fmt.Sprintf("remotePluginBug:%s", err.Error()))
		return resultFailed
	}

	errPrefix = "do request"
	resp, err := globalClient.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		panic(fmt.Errorf("not 2xx status code: %d", resp.StatusCode))
	}

	errPrefix = "read remote body"
	ctxBuff = rp.limitRead(resp.Body, maxContextBytes)

	errPrefix = "unmarshal context"
	rp.unmarshalHTTPContext(ctxBuff, ctx)

	return ""
}

// Status returns status.
func (rp *RemotePlugin) Status() interface{} { return nil }

// Close closes RemotePlugin.
func (rp *RemotePlugin) Close() {}

func (rp *RemotePlugin) marshalHTTPContext(ctx context.HTTPContext, reqBody, respBody []byte) []byte {
	r, w := ctx.Request(), ctx.Response()
	ctxEntity := contextEntity{
		Request: requestEntity{
			RealIP:   r.RealIP(),
			Method:   r.Method(),
			Scheme:   r.Scheme(),
			Host:     r.Host(),
			Path:     r.Path(),
			Query:    r.Query(),
			Fragment: r.Fragment(),
			Proto:    r.Proto(),
			Header:   r.Header().Std(),
			Body:     reqBody,
		},
		Response: responseEntity{
			StatusCode: w.StatusCode(),
			Header:     w.Header().Std(),
			Body:       respBody,
		},
	}

	buff, err := json.Marshal(ctxEntity)
	if err != nil {
		panic(err)
	}

	return buff
}

func (rp *RemotePlugin) unmarshalHTTPContext(buff []byte, ctx context.HTTPContext) {
	ctxEntity := &contextEntity{}

	err := json.Unmarshal(buff, ctxEntity)
	if err != nil {
		panic(err)
	}

	r, w := ctx.Request(), ctx.Response()
	re, we := ctxEntity.Request, ctxEntity.Response

	r.SetMethod(re.Method)
	r.SetPath(re.Path)
	r.Header().Reset(re.Header)
	r.SetBody(bytes.NewReader(re.Body))

	w.SetStatusCode(we.StatusCode)
	w.Header().Reset(we.Header)
	w.SetBody(bytes.NewReader(we.Body))
}
