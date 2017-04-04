package plugins

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"common"
	"logger"
	"pipelines"
	"task"
)

type httpOutputConfig struct {
	CommonConfig
	URLPattern               string            `json:"url_pattern"`
	HeaderPatterns           map[string]string `json:"header_patterns"`
	RequestBodyIOKey         string            `json:"request_body_io_key"`
	RequestBodyBufferPattern string            `json:"request_body_buffer_pattern"`
	Method                   string            `json:"method"`
	TimeoutSec               uint16            `json:"timeout_sec"` // up to 65535, zero means no timeout
	CertFile                 string            `json:"cert_file"`
	KeyFile                  string            `json:"key_file"`
	CAFile                   string            `json:"ca_file"`
	Insecure                 bool              `json:"insecure_tls"`

	ResponseCodeKey   string `json:"response_code_key"`
	ResponseBodyIOKey string `json:"response_body_io_key"`

	urlTokens, bodyTokens               []string
	headerNameTokens, headerValueTokens [][]string
	cert                                *tls.Certificate
	caCert                              []byte
}

func HTTPOutputConfigConstructor() Config {

	return &httpOutputConfig{
		TimeoutSec: 120,
	}
}

func (c *httpOutputConfig) Prepare() error {
	err := c.CommonConfig.Prepare()
	if err != nil {
		return err
	}

	ts := strings.TrimSpace
	c.URLPattern = ts(c.URLPattern)
	c.RequestBodyIOKey = ts(c.RequestBodyIOKey)
	c.RequestBodyBufferPattern = ts(c.RequestBodyBufferPattern)
	c.Method = ts(c.Method)
	c.CertFile = ts(c.CertFile)
	c.KeyFile = ts(c.KeyFile)
	c.CAFile = ts(c.CAFile)
	c.ResponseCodeKey = ts(c.ResponseCodeKey)
	c.ResponseBodyIOKey = ts(c.ResponseBodyIOKey)

	uri, err := url.ParseRequestURI(c.URLPattern)

	if err != nil || !uri.IsAbs() || uri.Hostname() == "" ||
		!common.StrInSlice(uri.Scheme, []string{"http", "https"}) {

		return fmt.Errorf("invalid url")
	}

	c.urlTokens, err = common.ScanTokens(c.URLPattern)
	if err != nil {
		return fmt.Errorf("invalid url pattern")
	}

	c.headerNameTokens = make([][]string, len(c.HeaderPatterns))
	c.headerValueTokens = make([][]string, len(c.HeaderPatterns))
	i := 0
	for name, value := range c.HeaderPatterns {
		if len(ts(name)) == 0 {
			return fmt.Errorf("invalid header name")
		}

		tokens, err := common.ScanTokens(name)
		if err != nil {
			return fmt.Errorf("invalid header name pattern")
		}
		c.headerNameTokens[i] = tokens

		tokens, err = common.ScanTokens(value)
		if err != nil {
			return fmt.Errorf("invalid header value pattern")
		}
		c.headerValueTokens[i] = tokens

		i++
	}

	_, ok := supportedMethods[c.Method]
	if !ok {
		return fmt.Errorf("invalid http method")
	}

	if c.TimeoutSec == 0 {
		logger.Warnf("[ZERO timeout has been applied, no request could be cancelled by timeout!]")
	}

	c.bodyTokens, err = common.ScanTokens(c.RequestBodyBufferPattern)
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
	conf   *httpOutputConfig
	client *http.Client
}

func HTTPOutputConstructor(conf Config) (Plugin, error) {
	c, ok := conf.(*httpOutputConfig)
	if !ok {
		return nil, fmt.Errorf("config type want *httpOutputConfig got %T", conf)
	}

	tlsConfig := new(tls.Config)
	tlsConfig.InsecureSkipVerify = c.Insecure

	h := &httpOutput{
		conf: c,
		client: &http.Client{
			Timeout:   time.Duration(c.TimeoutSec) * time.Second,
			Transport: &http.Transport{TLSClientConfig: tlsConfig},
		},
	}

	if c.cert != nil || c.caCert != nil {
		if c.cert != nil {
			tlsConfig.Certificates = []tls.Certificate{*c.cert}
			tlsConfig.BuildNameToCertificate()
		}

		if c.caCert != nil {
			caCertPool := x509.NewCertPool()
			caCertPool.AppendCertsFromPEM(c.caCert)
			tlsConfig.RootCAs = caCertPool
		}
	}

	return h, nil
}

func (h *httpOutput) Prepare(ctx pipelines.PipelineContext) {
	// Nothing to do.
}

func (h *httpOutput) send(t task.Task, req *http.Request) (*http.Response, error) {
	r := make(chan *http.Response, 1)
	e := make(chan error, 1)

	defer close(r)
	defer close(e)

	cancelCtx, cancel := context.WithCancel(context.Background())
	req.WithContext(cancelCtx)

	go func() {
		resp, err := h.client.Do(req)
		if err != nil {
			e <- err
		}
		r <- resp
	}()

	select {
	case resp := <-r:
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
	url := replacePatternWithTaskValue(t, h.conf.URLPattern, h.conf.urlTokens)

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
			length, _ = strconv.ParseInt(clen, 10, 64)
			reader = io.LimitReader(input, length)
		} else {
			// Request.ContentLength of 0 means either actually 0 or unknown
			reader = input
		}
	} else {
		body := replacePatternWithTaskValue(t, h.conf.RequestBodyBufferPattern, h.conf.bodyTokens)
		reader = bytes.NewBuffer([]byte(body))
		length = int64(len(body))
	}

	req, err := http.NewRequest(h.conf.Method, url, reader)
	if err != nil {
		t.SetError(err, task.ResultInternalServerError)
		return t, nil
	}
	req.ContentLength = length

	var headerNames, headerValues []string
	i := 0
	for name, value := range h.conf.HeaderPatterns {
		headerNames = append(headerNames, replacePatternWithTaskValue(t, name, h.conf.headerNameTokens[i]))
		headerValues = append(headerValues, replacePatternWithTaskValue(t, value, h.conf.headerValueTokens[i]))
		i++
	}

	for i > 0 {
		i--
		req.Header.Set(headerNames[i], headerValues[i])
	}

	req.Header.Set("User-Agent", "EaseGateway")

	resp, err := h.send(t, req)
	if err != nil {
		return t, nil
	}

	if len(h.conf.ResponseCodeKey) != 0 {
		t, err = task.WithValue(t, h.conf.ResponseCodeKey, resp.StatusCode)
		if err != nil {
			t.SetError(err, task.ResultInternalServerError)
			return t, nil
		}
	}

	if len(h.conf.ResponseBodyIOKey) != 0 {
		t, err = task.WithValue(t, h.conf.ResponseBodyIOKey, resp.Body)
		if err != nil {
			t.SetError(err, task.ResultInternalServerError)
			return t, nil
		}
	}

	closeHttpOutputResponseBody := func(t1 task.Task, _ task.TaskStatus) {
		t1.DeleteFinishedCallback(fmt.Sprintf("%s-closeHttpOutputResponseBody", h.Name()))

		resp.Body.Close()
	}

	t.AddFinishedCallback(fmt.Sprintf("%s-closeHttpOutputResponseBody", h.Name()), closeHttpOutputResponseBody)

	return t, nil
}

func (h *httpOutput) Name() string {
	return h.conf.PluginName()
}

func (h *httpOutput) Close() {
	// Nothing to do.
}

////

func replacePatternWithTaskValue(t task.Task, pattern string, tokens []string) string {
	ret := pattern
	for _, token := range tokens {
		var s string
		v := t.Value(token)
		if v != nil {
			s = task.ToString(v)
		}

		ret = strings.Replace(ret, fmt.Sprintf("{%s}", token), s, -1)
	}
	return ret
}
