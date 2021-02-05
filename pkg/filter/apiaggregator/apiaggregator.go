package apiaggregator

import (
	"bytes"
	stdcontext "context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"time"

	jsoniter "github.com/json-iterator/go"
	"github.com/megaease/easegateway/pkg/context"
	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/object/httppipeline"
	"github.com/megaease/easegateway/pkg/object/httpserver"
	"github.com/megaease/easegateway/pkg/supervisor"
	"github.com/megaease/easegateway/pkg/tracing"
	"github.com/megaease/easegateway/pkg/util/httpheader"
	"github.com/megaease/easegateway/pkg/util/pathadaptor"
)

const (
	// Kind is the kind of APIAggregator.
	Kind = "APIAggregator"

	resultFailed = "failed"
)

func init() {
	httppipeline.Register(&httppipeline.FilterRecord{
		Kind:            Kind,
		DefaultSpecFunc: DefaultSpec,
		NewFunc:         New,
		Results:         []string{resultFailed},
	})
}

// DefaultSpec returns default spec.
func DefaultSpec() *Spec {
	return &Spec{
		Timeout:      "60s",
		MaxBodyBytes: 10240,
	}
}

type (
	// APIAggregator is the entity to complete rate limiting.
	APIAggregator struct {
		spec *Spec
	}

	// Spec describes APIAggregator.
	Spec struct {
		httppipeline.FilterMeta `yaml:",inline"`

		// MaxBodyBytes in [0, 10MB]
		MaxBodyBytes   int64  `yaml:"maxBodyBytes" jsonschema:"omitempty,minimum=0,maximum=102400"`
		PartialSucceed bool   `yaml:"partialSucceed"`
		Timeout        string `yaml:"timeout" jsonschema:"omitempty,format=duration"`
		MergeResponse  bool   `yaml:"mergeResponse"`

		// User describes HTTP service target via an existing HTTPProxy
		APIProxys []*APIProxy `yaml:"apiproxys" jsonschema:"required"`

		timeout *time.Duration
	}

	// APIProxy describes the single API in EG's HTTPProxy object.
	APIProxy struct {
		// HTTPProxy's name in EG
		HTTPProxyName string `yaml:"httpproxyname" jsonschema:"required"`

		// Describes details about the request-target
		Method      string                `yaml:"method" jsonschema:"omitempty,format=httpmethod"`
		Path        *pathadaptor.Spec     `yaml:"path,omitempty" jsonschema:"omitempty"`
		Header      *httpheader.AdaptSpec `yaml:"header,omitempty" jsonschema:"omitempty"`
		DisableBody bool                  `yaml:"disableBody" jsonschema:"omitempty"`

		pa *pathadaptor.PathAdaptor
	}

	// Status contains status info of APIAggregator.
	Status struct {
	}
)

// New creates a APIAggregator.
func New(spec *Spec, prev *APIAggregator) *APIAggregator {
	if spec.Timeout != "" {
		timeout, err := time.ParseDuration(spec.Timeout)
		if err != nil {
			logger.Errorf("BUG: parse duration %s failed: %v",
				spec.Timeout, err)
		} else {
			spec.timeout = &timeout
		}
	}

	for _, proxy := range spec.APIProxys {

		if proxy.Path != nil {
			proxy.pa = pathadaptor.New(proxy.Path)
		}
	}

	return &APIAggregator{
		spec: spec,
	}
}

// Handle limits HTTPContext.
func (aa *APIAggregator) Handle(ctx context.HTTPContext) (result string) {

	buff := bytes.NewBuffer(nil)
	if aa.spec.MaxBodyBytes > 0 {
		written, err := io.CopyN(buff, ctx.Request().Body(), aa.spec.MaxBodyBytes+1)
		if written > aa.spec.MaxBodyBytes {
			ctx.AddTag(fmt.Sprintf("apiAggregator: request body exceed %dB", aa.spec.MaxBodyBytes))
			return resultFailed
		}
		if err != io.EOF {
			ctx.AddTag((fmt.Sprintf("apiAggregator: read request body failed: %v", err)))
			return resultFailed
		}
	}

	wg := &sync.WaitGroup{}
	wg.Add(len(aa.spec.APIProxys))

	httpResps := make([]context.HTTPReponse, len(aa.spec.APIProxys))
	// Using supervisor to call HTTPProxy object's Handle function
	for i, proxy := range aa.spec.APIProxys {
		req, err := aa.newHTTPReq(ctx, proxy, buff)

		if err != nil {
			logger.Errorf("BUG: new HTTPProxy request failed %v proxyname[%d]", err, aa.spec.APIProxys[i].HTTPProxyName)
			return resultFailed
		}

		go func(i int, name string, req *http.Request) {
			defer wg.Done()
			copyCtx, err := aa.newCtx(ctx, req, buff)

			if err != nil {
				httpResps[i] = nil
				return
			}
			ro, exists := supervisor.Global.GetRunningObject(name, supervisor.CategoryPipeline)
			if !exists {
				httpResps[i] = nil
			} else {
				handler, ok := ro.Instance().(httpserver.HTTPHandler)
				if !ok {
					httpResps[i] = nil
				} else {
					handler.Handle(copyCtx)
					httpResps[i] = copyCtx.Response()
				}
			}

		}(i, proxy.HTTPProxyName, req)
	}

	wg.Wait()

	for _, resp := range httpResps {
		_resp := resp

		if resp != nil {
			if body, ok := _resp.Body().(io.ReadCloser); ok {
				defer body.Close()
			}
		}
	}

	data := make(map[string][]byte)

	// Get all HTTPProxy response' body
	for i, resp := range httpResps {
		if resp == nil && !aa.spec.PartialSucceed {
			ctx.AddTag(fmt.Sprintf("apiAggregator: failed in HTTPProxy %s",
				aa.spec.APIProxys[i].HTTPProxyName))
			return resultFailed
		}

		// call HTTPProxy could be succesfful even with a no exist backend
		// so resp is not nil, but resp.Body() is nil.
		if resp != nil && resp.Body() != nil {
			if res := aa.copyHTTPBody2Map(resp.Body(), ctx, data, aa.spec.APIProxys[i].HTTPProxyName); len(res) != 0 {
				return res
			}
		}
	}

	return aa.formatResponse(ctx, data)

}

func (aa *APIAggregator) newCtx(ctx context.HTTPContext, req *http.Request, buff *bytes.Buffer) (context.HTTPContext, error) {
	// Construct a new context for the HTTPProxy
	// responseWriter is an HTTP responseRecorder, no the original context's real
	// repsonseWriter, or these Proxys will overwriten each others
	w := httptest.NewRecorder()
	var stdctx stdcontext.Context = ctx
	if aa.spec.timeout != nil {
		stdctx, _ = stdcontext.WithTimeout(stdctx, *aa.spec.timeout)
	}

	copyCtx := context.New(w, req, tracing.NoopTracing, "no trace")

	return copyCtx, nil
}

func (aa *APIAggregator) newHTTPReq(ctx context.HTTPContext, proxy *APIProxy, buff *bytes.Buffer) (*http.Request, error) {
	var stdctx stdcontext.Context = ctx
	if aa.spec.timeout != nil {
		// NOTE: Cancel function could be omiited here.
		stdctx, _ = stdcontext.WithTimeout(stdctx, *aa.spec.timeout)
	}

	method := ctx.Request().Method()
	if proxy.Method != "" {
		method = proxy.Method
	}

	url := ctx.Request().Std().URL
	if proxy.pa != nil {
		url.Path = proxy.pa.Adapt(url.Path)
	}

	var body io.Reader
	if !proxy.DisableBody {
		body = bytes.NewReader(buff.Bytes())
	}

	return http.NewRequestWithContext(stdctx, method, url.String(), body)

}

func (aa *APIAggregator) copyHTTPBody2Map(body io.Reader, ctx context.HTTPContext, data map[string][]byte, name string) string {
	respBody := bytes.NewBuffer(nil)

	written, err := io.CopyN(respBody, body, aa.spec.MaxBodyBytes)
	if written > aa.spec.MaxBodyBytes {
		ctx.AddTag(fmt.Sprintf("apiAggregator: response body exceed %dB", aa.spec.MaxBodyBytes))
		return resultFailed
	}
	if err != io.EOF {
		ctx.AddTag(fmt.Sprintf("apiAggregator: read response body failed: %v", err))
		return resultFailed
	}

	data[name] = respBody.Bytes()

	return ""
}

func (aa *APIAggregator) formatResponse(ctx context.HTTPContext, data map[string][]byte) string {
	if aa.spec.MergeResponse {
		result := map[string]interface{}{}
		for _, resp := range data {
			err := jsoniter.Unmarshal(resp, &result)
			if err != nil {
				ctx.AddTag(fmt.Sprintf("apiAggregator: unmarshal %s to json object failed: %v",
					resp, err))
				return resultFailed
			}
		}
		buff, err := jsoniter.Marshal(result)
		if err != nil {
			ctx.AddTag(fmt.Sprintf("apiAggregator: marshal %#v to json failed: %v",
				result, err))
			logger.Errorf("apiAggregator: marshal %#v to json failed: %v", result, err)
			return resultFailed
		}

		ctx.Response().SetBody(bytes.NewReader(buff))
	} else {
		result := []map[string]interface{}{}
		for _, resp := range data {
			ele := map[string]interface{}{}
			err := jsoniter.Unmarshal(resp, &ele)
			if err != nil {
				ctx.AddTag(fmt.Sprintf("apiAggregator: unmarshal %s to json object failed: %v",
					resp, err))
				return resultFailed
			}
			result = append(result, ele)
		}
		buff, err := jsoniter.Marshal(result)
		if err != nil {
			ctx.AddTag(fmt.Sprintf("apiAggregator: marshal %#v to json failed: %v",
				result, err))
			return resultFailed
		}

		ctx.Response().SetBody(bytes.NewReader(buff))
	}

	return ""
}

// Status returns APIAggregator status.
func (aa *APIAggregator) Status() *Status {
	return nil
}

// Close closes APIAggregator.
func (aa *APIAggregator) Close() {
}
