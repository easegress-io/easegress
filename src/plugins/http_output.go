package plugins

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/hexdecteam/easegateway-types/pipelines"
	"github.com/hexdecteam/easegateway-types/plugins"
	"github.com/hexdecteam/easegateway-types/task"

	"common"
	"logger"
	"option"
)

type httpOutputConfig struct {
	common.PluginCommonConfig
	URLPattern               string            `json:"url_pattern"`
	HeaderPatterns           map[string]string `json:"header_patterns"`
	Close                    bool              `json:"close_body_after_pipeline"`
	RequestBodyBufferPattern string            `json:"request_body_buffer_pattern"`
	Method                   string            `json:"method"`
	ExpectedResponseCodes    []int             `json:"expected_response_codes"`
	TimeoutSec               uint16            `json:"timeout_sec"` // up to 65535, zero means no timeout
	CertFile                 string            `json:"cert_file"`
	KeyFile                  string            `json:"key_file"`
	CAFile                   string            `json:"ca_file"`
	Insecure                 bool              `json:"insecure_tls"`
	DumpRequest              string            `json:"dump_request"`
	DumpResponse             string            `json:"dump_response"`

	RequestBodyIOKey  string `json:"request_body_io_key"`
	ResponseCodeKey   string `json:"response_code_key"`
	ResponseBodyIOKey string `json:"response_body_io_key"`

	cert              *tls.Certificate
	caCert            []byte
	dumpReq, dumpResp bool
}

func HTTPOutputConfigConstructor() plugins.Config {
	return &httpOutputConfig{
		TimeoutSec:            120,
		ExpectedResponseCodes: []int{http.StatusOK},
		DumpRequest:           "auto",
		DumpResponse:          "auto",
	}
}

func (c *httpOutputConfig) Prepare(pipelineNames []string) error {
	err := c.PluginCommonConfig.Prepare(pipelineNames)
	if err != nil {
		return err
	}

	ts := strings.TrimSpace
	c.URLPattern = ts(c.URLPattern)
	c.RequestBodyIOKey = ts(c.RequestBodyIOKey)
	c.Method = ts(c.Method)
	c.CertFile = ts(c.CertFile)
	c.KeyFile = ts(c.KeyFile)
	c.CAFile = ts(c.CAFile)
	c.DumpRequest = ts(c.DumpRequest)
	c.DumpResponse = ts(c.DumpResponse)
	c.ResponseCodeKey = ts(c.ResponseCodeKey)
	c.ResponseBodyIOKey = ts(c.ResponseBodyIOKey)

	uri, err := url.ParseRequestURI(c.URLPattern)
	if err != nil || !uri.IsAbs() || uri.Hostname() == "" ||
		!common.StrInSlice(uri.Scheme, []string{"http", "https"}) {

		return fmt.Errorf("invalid url")
	}

	_, err = common.ScanTokens(c.URLPattern, false, nil)
	if err != nil {
		return fmt.Errorf("invalid url pattern")
	}

	for name, value := range c.HeaderPatterns {
		if len(ts(name)) == 0 {
			return fmt.Errorf("invalid header name")
		}

		_, err := common.ScanTokens(name, false, nil)
		if err != nil {
			return fmt.Errorf("invalid header name pattern")
		}

		_, err = common.ScanTokens(value, false, nil)
		if err != nil {
			return fmt.Errorf("invalid header value pattern")
		}
	}

	_, err = common.ScanTokens(c.Method, false, nil)
	if err != nil {
		return fmt.Errorf("invalid method pattern")
	}

	if c.TimeoutSec == 0 {
		logger.Warnf("[ZERO timeout has been applied, no request could be cancelled by timeout!]")
	}

	dumpFlag := func(flag, name string) (bool, error) {
		if strings.ToLower(flag) == "auto" {
			return common.StrInSlice(option.Stage, []string{"debug", "test"}), nil
		} else if common.BoolFromStr(flag, false) {
			return true, nil
		} else if !common.BoolFromStr(flag, true) {
			return false, nil
		}

		return false, fmt.Errorf("invalid http %s dump option", name)
	}

	c.dumpReq, err = dumpFlag(c.DumpRequest, "request")
	if err != nil {
		return err
	}

	c.dumpResp, err = dumpFlag(c.DumpResponse, "response")
	if err != nil {
		return err
	}

	_, err = common.ScanTokens(c.RequestBodyBufferPattern, false, nil)
	if err != nil {
		return fmt.Errorf("invalid body buffer pattern")
	}

	if len(c.CertFile) != 0 && len(c.KeyFile) != 0 {
		cert, err := tls.LoadX509KeyPair(c.CertFile, c.KeyFile)
		if err != nil {
			return fmt.Errorf("invalid PEM eoncoded certificate and/or preivate key file(s)")
		}
		c.cert = &cert
	}

	if len(c.CAFile) != 0 {
		c.caCert, err = ioutil.ReadFile(c.CAFile)
		if err != nil {
			return fmt.Errorf("invalid PEM eoncoded CA certificate file")
		}
	}

	return nil
}

////

type httpOutput struct {
	conf       *httpOutputConfig
	instanceId string
	client     *http.Client
}

func HTTPOutputConstructor(conf plugins.Config) (plugins.Plugin, error) {
	c, ok := conf.(*httpOutputConfig)
	if !ok {
		return nil, fmt.Errorf("config type want *httpOutputConfig got %T", conf)
	}

	tlsConfig := new(tls.Config)
	tlsConfig.InsecureSkipVerify = c.Insecure

	h := &httpOutput{
		conf: c,
		client: &http.Client{
			Timeout: time.Duration(c.TimeoutSec) * time.Second,
			Transport: &http.Transport{
				Proxy: http.ProxyFromEnvironment,
				DialContext: (&net.Dialer{
					Timeout:   30 * time.Second,
					KeepAlive: 30 * time.Second,
					DualStack: true,
				}).DialContext,
				MaxIdleConns:          100,
				MaxIdleConnsPerHost:   20,
				IdleConnTimeout:       90 * time.Second,
				TLSHandshakeTimeout:   10 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
				TLSClientConfig:       tlsConfig,
			},
		},
	}

	h.instanceId = fmt.Sprintf("%p", h)

	if c.cert != nil {
		tlsConfig.Certificates = []tls.Certificate{*c.cert}
		tlsConfig.BuildNameToCertificate()
	}

	if c.caCert != nil {
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(c.caCert)
		tlsConfig.RootCAs = caCertPool
	}

	return h, nil
}

func (h *httpOutput) Prepare(ctx pipelines.PipelineContext) {
	// Nothing to do.
}

func (h *httpOutput) send(ctx pipelines.PipelineContext, t task.Task, req *http.Request) (*http.Response, error) {
	r := make(chan *http.Response)
	e := make(chan error)

	defer close(r)
	defer close(e)

	cancelCtx, cancel := context.WithCancel(context.Background())
	req = req.WithContext(cancelCtx)

	if h.conf.dumpReq {
		logger.HTTPReqDump(ctx.PipelineName(), h.Name(), h.instanceId, t.StartAt().UnixNano(), req)
	}

	go func() {
		defer func() {
			// channel e and r can be closed first before return by existing send()
			// caused by task cancellation, the result or error of Do() can be ignored safely.
			recover()
		}()

		resp, err := h.client.Do(req)
		if err != nil {
			e <- err
		} else {
			r <- resp
		}
	}()

	select {
	case resp := <-r:
		if h.conf.dumpResp {
			logger.HTTPRespDump(ctx.PipelineName(), h.Name(), h.instanceId, t.StartAt().UnixNano(), resp)
		}

		return resp, nil
	case err := <-e:
		t.SetError(err, task.ResultServiceUnavailable)
		return nil, err
	case <-t.Cancel():
		cancel()
		err := fmt.Errorf("task is cancelled by %s", t.CancelCause())
		t.SetError(err, task.ResultTaskCancelled)
		return nil, err
	}
}

func (h *httpOutput) Run(ctx pipelines.PipelineContext, t task.Task) (task.Task, error) {
	// skip error check safely due to we ensured it in Prepare()
	link, _ := ReplaceTokensInPattern(t, h.conf.URLPattern)

	var length int64
	var reader io.Reader
	if len(h.conf.RequestBodyIOKey) != 0 {
		inputValue := t.Value(h.conf.RequestBodyIOKey)
		input, ok := inputValue.(io.Reader)
		if !ok {
			t.SetError(fmt.Errorf("input %s got wrong value: %#v", h.conf.RequestBodyIOKey, inputValue),
				task.ResultMissingInput)
			return t, nil
		}

		// optimization and defensive for http proxy case
		lenValue := t.Value("HTTP_CONTENT_LENGTH")
		clen, ok := lenValue.(string)
		if ok {
			var err error
			length, err = strconv.ParseInt(clen, 10, 64)
			if err == nil && length >= 0 {
				reader = io.LimitReader(input, length)
			} else {
				reader = input
			}
		} else {
			// Request.ContentLength of 0 means either actually 0 or unknown
			reader = input
		}
	} else {
		// skip error check safely due to we ensured it in Prepare()
		body, _ := ReplaceTokensInPattern(t, h.conf.RequestBodyBufferPattern)
		reader = bytes.NewBuffer([]byte(body))
		length = int64(len(body))
	}

	// skip error check safely due to we ensured it in Prepare()
	method, _ := ReplaceTokensInPattern(t, h.conf.Method)

	_, ok := supportedMethods[method]
	if !ok {
		t.SetError(fmt.Errorf("invalid http method %s", method), task.ResultMissingInput)
		return t, nil
	}

	req, err := http.NewRequest(method, link, reader)
	if err != nil {
		t.SetError(err, task.ResultInternalServerError)
		return t, nil
	}

	req.ContentLength = length
	req.Header.Set("Connection", "keep-alive")

	i := 0
	for name, value := range h.conf.HeaderPatterns {
		// skip error check safely due to we ensured it in Prepare()
		name1, _ := ReplaceTokensInPattern(t, name)
		value1, _ := ReplaceTokensInPattern(t, value)
		req.Header.Set(name1, value1)
		i++
	}
	req.Header.Set("User-Agent", "EaseGateway")

	resp, err := h.send(ctx, t, req)
	if err != nil {
		return t, nil
	}

	closeRespBody := func() {
		closeHTTPOutputResponseBody := func(t1 task.Task, _ task.TaskStatus) {
			t1.DeleteFinishedCallback(fmt.Sprintf("%s-closeHTTPOutputResponseBody", h.Name()))

			err := resp.Body.Close()
			if err != nil {
				logger.Errorf("[close response body failed: %v]", err)
			}
		}

		t.AddFinishedCallback(fmt.Sprintf("%s-closeHTTPOutputResponseBody", h.Name()),
			closeHTTPOutputResponseBody)
	}

	if h.conf.Close || len(h.conf.ResponseBodyIOKey) == 0 {
		closeRespBody()
	}

	if len(h.conf.ResponseBodyIOKey) != 0 {
		t, err = task.WithValue(t, h.conf.ResponseBodyIOKey, resp.Body)
		if err != nil {
			closeRespBody()

			t.SetError(err, task.ResultInternalServerError)
			return t, nil
		}
	}

	if len(h.conf.ExpectedResponseCodes) > 0 {
		match := false
		for _, expected := range h.conf.ExpectedResponseCodes {
			if resp.StatusCode == expected {
				match = true
				break
			}
		}
		if !match {
			err = fmt.Errorf("http upstream responded with unexpected status code (%d)", resp.StatusCode)
			t.SetError(err, task.ResultServiceUnavailable)
			return t, nil
		}
	}

	if len(h.conf.ResponseCodeKey) != 0 {
		t, err = task.WithValue(t, h.conf.ResponseCodeKey, resp.StatusCode)
		if err != nil {
			t.SetError(err, task.ResultInternalServerError)
			return t, nil
		}
	}

	return t, nil
}

func (h *httpOutput) Name() string {
	return h.conf.PluginName()
}

func (h *httpOutput) Close() {
	// Nothing to do.
}
