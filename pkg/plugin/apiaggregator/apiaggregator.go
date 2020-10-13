package apiaggregator

import (
	"bytes"
	stdcontext "context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	jsoniter "github.com/json-iterator/go"
	"github.com/megaease/easegateway/pkg/context"
	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/object/httppipeline"
	"github.com/megaease/easegateway/pkg/util/httpheader"
	"github.com/megaease/easegateway/pkg/util/pathadaptor"
)

const (
	// Kind is the kind of APIAggregator.
	Kind = "APIAggregator"

	resultFailed = "failed"
)

func init() {
	httppipeline.Register(&httppipeline.PluginRecord{
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
		httppipeline.PluginMeta `yaml:",inline"`

		// MaxBodyBytes in [0, 10MB]
		MaxBodyBytes   int64      `yaml:"maxBodyBytes" jsonschema:"omitempty,minimum=0,maximum=102400"`
		PartialSucceed bool       `yaml:"partialSucceed"`
		Timeout        string     `yaml:"timeout" jsonschema:"omitempty,format=duration"`
		MergeResponse  bool       `yaml:"mergeResponse"`
		APIs           []*APISpec `yaml:"apis" jsonschema:"required"`

		timeout *time.Duration
	}

	// APISpec describes the single API.
	APISpec struct {
		URL         string                `yaml:"url" jsonschema:"required,format=url"`
		Method      string                `yaml:"method" jsonschema:"omitempty,format=httpmethod"`
		Path        *pathadaptor.Spec     `yaml:"path,omitempty" jsonschema:"omitempty"`
		Header      *httpheader.AdaptSpec `yaml:"header,omitempty" jsonschema:"omitempty"`
		DisableBody bool                  `yaml:"disableBody" jsonschema:"omitempty"`

		url *url.URL
		pa  *pathadaptor.PathAdaptor
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

	for _, api := range spec.APIs {
		_url, err := url.Parse(api.URL)
		if err != nil {
			logger.Errorf("BUG: parse url %s failed: %v", api.URL, err)
		} else {
			api.url = _url
		}

		if api.Path != nil {
			api.pa = pathadaptor.New(api.Path)
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

	reqs := make([]*http.Request, len(aa.spec.APIs))
	for i, api := range aa.spec.APIs {
		var stdctx stdcontext.Context = ctx
		if aa.spec.timeout != nil {
			// NOTE: Cancel function could be omiited here.
			stdctx, _ = stdcontext.WithTimeout(stdctx, *aa.spec.timeout)
		}

		method := ctx.Request().Method()
		if api.Method != "" {
			method = api.Method
		}

		url := *api.url
		if api.pa != nil {
			url.Path = api.pa.Adapt(url.Path)
		}

		var body io.Reader
		if !api.DisableBody {
			body = bytes.NewReader(buff.Bytes())
		}

		var err error
		reqs[i], err = http.NewRequestWithContext(stdctx, method, url.String(), body)
		if err != nil {
			logger.Errorf("BUG: new request failed %v", err)
			return resultFailed
		}
	}

	wg := &sync.WaitGroup{}
	wg.Add(len(reqs))
	resps := make([]*http.Response, len(reqs))
	for i, req := range reqs {
		go func(i int, req *http.Request) {
			defer wg.Done()
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				resps[i] = nil
			} else {
				resps[i] = resp
			}
		}(i, req)
	}
	wg.Wait()

	for _, resp := range resps {
		_resp := resp
		if resp != nil {
			defer _resp.Body.Close()
		}
	}

	var data map[string][]byte
	for i, resp := range resps {
		if resp == nil && !aa.spec.PartialSucceed {
			ctx.AddTag(fmt.Sprintf("apiAggregator: failed in api %s",
				aa.spec.APIs[i].url))
			return resultFailed
		}
		respBody := bytes.NewBuffer(nil)

		written, err := io.CopyN(respBody, resp.Body, aa.spec.MaxBodyBytes)
		if written > aa.spec.MaxBodyBytes {
			ctx.AddTag(fmt.Sprintf("apiAggregator: response body exceed %dB", aa.spec.MaxBodyBytes))
			return resultFailed
		}
		if err != io.EOF {
			ctx.AddTag(fmt.Sprintf("apiAggregator: read response body failed: %v", err))
			return resultFailed
		}

		data[aa.spec.APIs[i].URL] = respBody.Bytes()
	}

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
