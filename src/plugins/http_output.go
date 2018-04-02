package plugins

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hexdecteam/easegateway-types/pipelines"
	"github.com/hexdecteam/easegateway-types/plugins"
	"github.com/hexdecteam/easegateway-types/task"

	"common"
	eghttp "http"
	"logger"
	"option"
)

type HTTPOutputConfig struct {
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
	ConnKeepAlive            string            `json:"keepalive"`
	ConnKeepAliveSec         uint16            `json:"keepalive_sec"` // up to 65535
	DumpRequest              string            `json:"dump_request"`
	DumpResponse             string            `json:"dump_response"`
	Type                     string            `json:"type"`

	RequestBodyIOKey    string `json:"request_body_io_key"`
	RequestHeaderKey    string `json:"request_header_key"`
	ResponseCodeKey     string `json:"response_code_key"`
	ResponseBodyIOKey   string `json:"response_body_io_key"`
	ResponseHeaderKey   string `json:"response_header_key"`
	ResponseRemoteKey   string `json:"response_remote_key"`
	ResponseDurationKey string `json:"response_duration_key"`

	typ               plugins.HTTPType
	cert              *tls.Certificate
	caCert            []byte
	connKeepAlive     int16
	dumpReq, dumpResp bool
}

func HTTPOutputConfigConstructor() plugins.Config {
	return &HTTPOutputConfig{
		TimeoutSec:            120,
		ExpectedResponseCodes: []int{http.StatusOK},
		ConnKeepAlive:         "auto",
		ConnKeepAliveSec:      30,
		DumpRequest:           "auto",
		DumpResponse:          "auto",
		Type:                  eghttp.NetHTTPStr,
	}
}

func (c *HTTPOutputConfig) Prepare(pipelineNames []string) error {
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
	c.ResponseRemoteKey = ts(c.ResponseRemoteKey)
	c.ResponseDurationKey = ts(c.ResponseDurationKey)
	c.Type = ts(c.Type)

	_, err = common.ScanTokens(c.URLPattern, false, nil)
	if err != nil {
		return fmt.Errorf("scan pattern failed, invalid url pattern %s", c.URLPattern)
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

	if strings.ToLower(c.ConnKeepAlive) == "auto" {
		c.connKeepAlive = -1
	} else if common.BoolFromStr(c.ConnKeepAlive, false) {
		c.connKeepAlive = 1
	} else if !common.BoolFromStr(c.ConnKeepAlive, true) {
		c.connKeepAlive = 0
	} else {
		return fmt.Errorf("invalid connection keep-alive option")
	}

	if c.ConnKeepAliveSec == 0 {
		return fmt.Errorf("invalid connection keep-alive period")
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

	if len(c.CertFile) != 0 || len(c.KeyFile) != 0 {
		certFilePath := filepath.Join(common.CERT_HOME_DIR, c.CertFile)
		keyFilePath := filepath.Join(common.CERT_HOME_DIR, c.KeyFile)

		if s, err := os.Stat(certFilePath); os.IsNotExist(err) || s.IsDir() {
			return fmt.Errorf("cert file %s not found", c.CertFile)
		}

		if s, err := os.Stat(keyFilePath); os.IsNotExist(err) || s.IsDir() {
			return fmt.Errorf("key file %s not found", c.KeyFile)
		}

		cert, err := tls.LoadX509KeyPair(certFilePath, keyFilePath)
		if err != nil {
			return fmt.Errorf("invalid PEM eoncoded certificate and/or preivate key file")
		}
		c.cert = &cert
	}

	if len(c.CAFile) != 0 {
		caFilePath := filepath.Join(common.CERT_HOME_DIR, c.CAFile)

		if s, err := os.Stat(caFilePath); os.IsNotExist(err) || s.IsDir() {
			return fmt.Errorf("CA certificate file %s not found", c.CAFile)
		}

		c.caCert, err = ioutil.ReadFile(caFilePath)
		if err != nil {
			return fmt.Errorf("invalid PEM eoncoded CA certificate file")
		}
	}

	typ, err := eghttp.ParseHTTPType(c.Type)
	if err != nil {
		return fmt.Errorf("parse http implementation failed: %v", err)
	}
	c.typ = typ
	return nil
}

////

const HTTP_OUTPUT_CLIENT_COUNT = 20

var cancelledError = fmt.Errorf("task is canceled")

type httpOutput struct {
	conf       *HTTPOutputConfig
	instanceId string
	clients    []client
}

func httpOutputConstructor(conf plugins.Config) (plugins.Plugin, plugins.PluginType, bool, error) {
	c, ok := conf.(*HTTPOutputConfig)
	if !ok {
		return nil, plugins.SinkPlugin, false, fmt.Errorf("config type want *HTTPOutputConfig got %T", conf)
	}

	h := &httpOutput{
		conf:    c,
		clients: make([]client, HTTP_OUTPUT_CLIENT_COUNT),
	}

	for i := 0; i < HTTP_OUTPUT_CLIENT_COUNT; i++ {
		h.clients[i] = createClient(c)
	}

	h.instanceId = fmt.Sprintf("%p", h)

	return h, plugins.SinkPlugin, false, nil
}

func (h *httpOutput) Prepare(ctx pipelines.PipelineContext) {
	// Nothing to do.
}

func (h *httpOutput) send(ctx pipelines.PipelineContext, t task.Task, req request) (response, time.Duration, error) {

	if h.conf.dumpReq {
		logger.HTTPReqDump(ctx.PipelineName(), h.Name(), h.instanceId, t.StartAt().UnixNano(), req.Dump)
	}

	resp, duration, err := req.Do(t)
	if err != nil {
		if err == cancelledError {
			t.SetError(err, task.ResultTaskCancelled)
		} else {
			t.SetError(err, task.ResultServiceUnavailable)
		}
	} else {
		if h.conf.dumpResp {
			logger.HTTPRespDump(ctx.PipelineName(), h.Name(), h.instanceId, t.StartAt().UnixNano(), resp.Dump)
		}
	}

	return resp, duration, err
}

func (h *httpOutput) Run(ctx pipelines.PipelineContext, t task.Task) error {
	// skip error check safely due to we ensured it in Prepare()
	url, _ := ReplaceTokensInPattern(t, h.conf.URLPattern)

	var reader plugins.SizedReadCloser
	if len(h.conf.RequestBodyIOKey) != 0 {
		inputValue := t.Value(h.conf.RequestBodyIOKey)
		ok := false
		if reader, ok = inputValue.(plugins.SizedReadCloser); !ok {
			t.SetError(fmt.Errorf("input %s got wrong value: %#v", h.conf.RequestBodyIOKey, inputValue),
				task.ResultMissingInput)
			return nil
		}
	} else {
		// skip error check safely due to we ensured it in Prepare()
		body, _ := ReplaceTokensInPattern(t, h.conf.RequestBodyBufferPattern)
		byteBody := []byte(body)
		reader = common.NewSizedReadCloser(ioutil.NopCloser(bytes.NewBuffer(byteBody)), int64(len(byteBody)))
	}

	// skip error check safely due to we ensured it in Prepare()
	method, _ := ReplaceTokensInPattern(t, h.conf.Method)

	_, ok := supportedMethods[method]
	if !ok {
		t.SetError(fmt.Errorf("invalid http method %s", method), task.ResultMissingInput)
		return nil
	}
	client := h.clients[rand.Intn(HTTP_OUTPUT_CLIENT_COUNT)]

	req, err := client.CreateRequest(method, url, reader)
	if err != nil {
		t.SetError(err, task.ResultInternalServerError)
		return nil
	}

	if len(h.conf.RequestHeaderKey) != 0 {
		copyRequestHeaderFromTask(t, h.conf.RequestHeaderKey, req.Header())
	}

	if h.conf.connKeepAlive == 0 {
		req.Header().Set("Connection", "close")
	} else if h.conf.connKeepAlive == 1 {
		req.Header().Set("Connection", "keep-alive")
	} else { // h.conf.connKeepAlive == -1, auto mode
		// http proxy case
		keepAliveValue := t.Value("HTTP_CONNECTION")
		if keepAliveStr, ok := keepAliveValue.(string); ok {
			v := strings.TrimSpace(keepAliveStr)
			if strings.EqualFold(v, "keep-alive") {
				req.Header().Set("Connection", "keep-alive")
			} else {
				req.Header().Set("Connection", "close")
			}
		} else { // Connection header doesn't exist in original request
			// use default value of protocol:
			// HTTP/1.1, keep-alive is enabled by default
			// HTTP/1.0, keep-alive is disabled by default

			// https://tools.ietf.org/html/rfc3875#section-4.1.16
			serverProtocol := t.Value("SERVER_PROTOCOL")
			if protocol, ok := serverProtocol.(string); ok && protocol == "HTTP/1.0" {
				req.Header().Set("Connection", "close")
			}
		}
	}

	i := 0
	for name, value := range h.conf.HeaderPatterns {
		// skip error check safely due to we ensured it in Prepare()
		name1, _ := ReplaceTokensInPattern(t, name)
		value1, _ := ReplaceTokensInPattern(t, value)
		req.Header().Set(name1, value1)
		i++
	}
	resp, responseDuration, err := h.send(ctx, t, req)
	if err != nil && t.ResultCode() == task.ResultTaskCancelled &&
		len(h.conf.RequestBodyIOKey) == 0 { // has no rewind for rerun plugin
		return t.Error()
	}

	if len(h.conf.ResponseDurationKey) != 0 {
		t.WithValue(h.conf.ResponseDurationKey, responseDuration)
	}

	if err != nil {
		return nil
	}

	closeRespBody := func() {
		closeHTTPOutputResponseBody := func(t1 task.Task, _ task.TaskStatus) {
			err := resp.BodyReadCloser().Close()
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
		t.WithValue(h.conf.ResponseBodyIOKey, resp.BodyReadCloser())
	}

	if len(h.conf.ResponseHeaderKey) != 0 {
		t.WithValue(h.conf.ResponseHeaderKey, resp.Header())
	}

	if len(h.conf.ResponseCodeKey) != 0 {
		t.WithValue(h.conf.ResponseCodeKey, resp.StatusCode)
	}

	if len(h.conf.ResponseRemoteKey) != 0 {
		t.WithValue(h.conf.ResponseRemoteKey, url)
	}

	if len(h.conf.ExpectedResponseCodes) > 0 {
		match := false
		for _, expected := range h.conf.ExpectedResponseCodes {
			if resp.StatusCode() == expected {
				match = true
				break
			}
		}
		if !match {
			err = fmt.Errorf("http upstream responded with unexpected status code (%d)", resp.StatusCode())
			t.SetError(err, task.ResultServiceUnavailable)
			return nil
		}
	}

	return nil
}

func (h *httpOutput) Name() string {
	return h.conf.PluginName()
}

func (h *httpOutput) CleanUp(ctx pipelines.PipelineContext) {
	// Nothing to do.
}

func (h *httpOutput) Close() {
	// Nothing to do.
}

////
func init() {
	rand.Seed(time.Now().UnixNano())
}
