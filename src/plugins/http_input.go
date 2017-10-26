package plugins

import (
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/hexdecteam/easegateway-types/pipelines"
	"github.com/hexdecteam/easegateway-types/plugins"
	"github.com/hexdecteam/easegateway-types/task"

	"common"
	"logger"
	"option"
)

// TODO: Moves this into plugin, aligns https server life-cycle with plugin life-cycle
func init() {
	if option.ShowVersion {
		return
	}

	var (
		disableHTTPS bool
		certPath     string
		keyPath      string
	)

	if len(option.CertFile) == 0 || len(option.KeyFile) == 0 {
		if len(option.CertFile) != 0 {
			logger.Infof("[keyfile set empty, certfile set: %s]", option.CertFile)
		} else if len(option.KeyFile) != 0 {
			logger.Infof("[certfile set empty, keyfile set: %s]", option.KeyFile)
		} else {
			logger.Infof("[certfile and keyfile set empty]")
		}
		disableHTTPS = true
	} else {
		certPath = filepath.Join(common.CERT_HOME_DIR, option.CertFile)
		keyPath = filepath.Join(common.CERT_HOME_DIR, option.KeyFile)
		logger.Infof("[cert file: %s]", certPath)
		logger.Infof("[key file: %s]", keyPath)

		if _, err := os.Stat(certPath); os.IsNotExist(err) {
			logger.Warnf("[certfile not found]")
			disableHTTPS = true
		}

		if _, err := os.Stat(keyPath); os.IsNotExist(err) {
			logger.Warnf("[key file not found]")
			disableHTTPS = true
		}
	}

	keepAliveTimeout := 10 * time.Second // TODO: move it to httpInputConfig
	// TODO: Adds keep_alive_requests and max_connections to httpInputConfig after making http server plugin-life-cycle

	srv := &http.Server{
		Handler:     defaultMux,
		IdleTimeout: keepAliveTimeout,
	}

	if disableHTTPS {
		addr := fmt.Sprintf("%s:10080", option.Host)
		logger.Infof("[downgrade HTTPS to HTTP, listen %s]", addr)
		srv.Addr = addr
		go func() {
			err := srv.ListenAndServe()
			if err != nil {
				logger.Errorf("[listen failed: %v]", err)
			}
		}()
	} else {
		addr := fmt.Sprintf("%s:10443", option.Host)
		logger.Infof("[upgrade HTTP to HTTPS, listen %s]", addr)
		srv.Addr = addr
		go func() {
			err := srv.ListenAndServeTLS(certPath, keyPath)
			if err != nil {
				logger.Errorf("[listen failed: %v]", err)
			}
		}()
	}
}

////

type httpInputConfig struct {
	common.PluginCommonConfig
	URL         string              `json:"url"`
	Methods     []string            `json:"methods"`
	HeadersEnum map[string][]string `json:"headers_enum"`
	Unzip       bool                `json:"unzip"`
	RespondErr  bool                `json:"respond_error"`
	FastClose   bool                `json:"fast_close"`
	DumpRequest string              `json:"dump_request"`

	RequestHeaderNamesKey string `json:"request_header_names_key"`
	RequestBodyIOKey      string `json:"request_body_io_key"`
	ResponseCodeKey       string `json:"response_code_key"`
	ResponseBodyIOKey     string `json:"response_body_io_key"`
	ResponseBodyBufferKey string `json:"response_body_buffer_key"`

	dumpReq bool
}

func HTTPInputConfigConstructor() plugins.Config {
	return &httpInputConfig{
		Methods:     []string{http.MethodGet},
		Unzip:       true,
		DumpRequest: "auto",
	}
}

func (c *httpInputConfig) Prepare(pipelineNames []string) error {
	err := c.PluginCommonConfig.Prepare(pipelineNames)
	if err != nil {
		return err
	}

	ts := strings.TrimSpace
	c.URL = ts(c.URL)
	for i := range c.Methods {
		c.Methods[i] = ts(c.Methods[i])
	}

	c.DumpRequest = ts(c.DumpRequest)
	c.RequestHeaderNamesKey = ts(c.RequestHeaderNamesKey)
	c.RequestBodyIOKey = ts(c.RequestBodyIOKey)
	c.ResponseCodeKey = ts(c.ResponseCodeKey)
	c.ResponseBodyIOKey = ts(c.ResponseBodyIOKey)
	c.ResponseBodyBufferKey = ts(c.ResponseBodyBufferKey)

	if !filepath.IsAbs(c.URL) {
		return fmt.Errorf("invalid relative url")
	}

	var positions []int
	token_booker := func(pos int, token string) (care bool, replacement string) {
		positions = append(positions, pos)
		positions = append(positions, pos+len(token)+1)
		return false, ""
	}

	_, err = common.ScanTokens(c.URL,
		false, /* do not remove escape char, due to escape char is not allowed in the path and pattern */
		token_booker)
	if err != nil {
		return err
	}

	// one and only one parameter fits in a segment of the path
	// e.g. correct: `/{a}/{b}`, wrong: `/{a}b{c}` and `/{a}b`
	for _, pos := range positions {
		if pos == 0 { // defensive, not an absolute path
			return fmt.Errorf("invalid parametric url")
		}

		if []byte(c.URL)[pos] == '{' {
			if []byte(c.URL)[pos-1] != '/' {
				return fmt.Errorf("invalid parametric url")
			}
		} else { // []byte(c.URL)[pos] == '}'
			if pos+1 < len(c.URL) && []byte(c.URL)[pos+1] != '/' {
				return fmt.Errorf("invalid parametric url")
			}
		}
	}

	methodMarks := map[string]bool{}
	for _, method := range c.Methods {
		_, ok := supportedMethods[method]
		if !ok {
			return fmt.Errorf("invalid http method")
		}
		if methodMarks[method] {
			return fmt.Errorf("duplicated http method: %s", method)
		}
		methodMarks[method] = true
	}

	for key, value := range c.HeadersEnum {
		key = ts(key)
		if len(key) == 0 {
			return fmt.Errorf("invalid http headers enum")
		}
		c.HeadersEnum[key] = value
	}

	if strings.ToLower(c.DumpRequest) == "auto" {
		c.dumpReq = common.StrInSlice(option.Stage, []string{"debug", "test"})
	} else if common.BoolFromStr(c.DumpRequest, false) {
		c.dumpReq = true
	} else if !common.BoolFromStr(c.DumpRequest, true) {
		c.dumpReq = false
	} else {
		return fmt.Errorf("invalid http request dump option")
	}

	return nil
}

////

type httpTask struct {
	request      *http.Request
	writer       http.ResponseWriter
	receivedAt   time.Time
	path_params  map[string]string
	finishedChan chan struct{}
}

////

type httpInput struct {
	conf         *httpInputConfig
	httpTaskChan chan *httpTask
	instanceId   string
	queueLength  uint64
}

func HTTPInputConstructor(conf plugins.Config) (plugins.Plugin, error) {
	c, ok := conf.(*httpInputConfig)
	if !ok {
		return nil, fmt.Errorf("config type want *HTTPInputConfig got %T", conf)
	}

	h := &httpInput{
		conf:         c,
		httpTaskChan: make(chan *httpTask, 32767),
	}

	h.instanceId = fmt.Sprintf("%p", h)

	for _, method := range h.conf.Methods {
		err := defaultMux.HandleFunc(h.conf.URL, method, h.conf.HeadersEnum, h.handler)
		if err != nil {
			return nil, fmt.Errorf("add handler failed: %v", err)
		}
	}

	return h, nil
}

func (h *httpInput) Prepare(ctx pipelines.PipelineContext) {
	added, err := ctx.Statistics().RegisterPluginIndicator(h.Name(), h.instanceId, "WAIT_QUEUE_LENGTH",
		"The length of wait queue which contains requests wait to be handled by a pipeline.",
		func(pluginName, indicatorName string) (interface{}, error) {
			return h.queueLength, nil
		})
	if err != nil {
		logger.Warnf("[BUG: register plugin %s indicator %s failed, "+
			"ignored to expose customized statistics indicator: %s]", h.Name(), "WAIT_QUEUE_LENGTH", err)
	} else if added {
		ctx.Statistics().UnregisterPluginIndicatorAfterPluginDelete(
			h.Name(), h.instanceId, "WAIT_QUEUE_LENGTH")
		ctx.Statistics().UnregisterPluginIndicatorAfterPluginUpdate(
			h.Name(), h.instanceId, "WAIT_QUEUE_LENGTH")
	}

	added, err = ctx.Statistics().RegisterPluginIndicator(h.Name(), h.instanceId, "WIP_REQUEST_COUNT",
		"The count of request which in the working progress of the pipeline.",
		func(pluginName, indicatorName string) (interface{}, error) {
			wipReqCount, err := getHTTPInputHandlingRequestCount(ctx, pluginName)
			if err != nil {
				return nil, err
			}
			return *wipReqCount, nil
		})
	if err != nil {
		logger.Warnf("[BUG: register plugin %s indicator %s failed, "+
			"ignored to expose customized statistics indicator: %s]", h.Name(), "WIP_REQUEST_COUNT", err)
	} else if added {
		ctx.Statistics().UnregisterPluginIndicatorAfterPluginDelete(
			h.Name(), h.instanceId, "WIP_REQUEST_COUNT")
		ctx.Statistics().UnregisterPluginIndicatorAfterPluginUpdate(
			h.Name(), h.instanceId, "WIP_REQUEST_COUNT")
	}
}

func (h *httpInput) handler(w http.ResponseWriter, req *http.Request, path_params map[string]string) {
	if h.conf.Unzip && strings.Contains(req.Header.Get("Content-Encoding"), "gzip") {
		var err error
		req.Body, err = gzip.NewReader(req.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	}

	httpTask := httpTask{
		request:      req,
		writer:       w,
		receivedAt:   time.Now(),
		path_params:  path_params,
		finishedChan: make(chan struct{}),
	}

	func() {
		defer func() {
			// To allow concurrent request handling, there's no lock in the plugin,
			// which caused a race condition between channel write (here) and close (in Close())
			// use this way to ignore error since plugin will exit in anyway, and notice client.
			err := recover()
			if err != nil {
				w.WriteHeader(http.StatusBadGateway)
			}
		}()
		h.httpTaskChan <- &httpTask
		atomic.AddUint64(&h.queueLength, 1)
	}()

	<-httpTask.finishedChan
}

func (h *httpInput) receive(ctx pipelines.PipelineContext, t task.Task) (error, task.TaskResultCode, task.Task) {
	for {
		var ok bool
		var ht *httpTask
		var err error

		select {
		case ht, ok = <-h.httpTaskChan:
			if !ok {
				return fmt.Errorf("plugin %s has been closed", h.Name()),
					task.ResultInternalServerError, t
			}
			for !atomic.CompareAndSwapUint64(&h.queueLength, h.queueLength, h.queueLength-1) {
			}
		case <-t.Cancel():
			return fmt.Errorf("task is cancelled by %s", t.CancelCause()), task.ResultTaskCancelled, t
		}

		if h.conf.dumpReq {
			logger.HTTPReqDump(ctx.PipelineName(), h.Name(), h.instanceId, t.StartAt().UnixNano(), ht.request)
		}

		respondCaller := func(t1 task.Task, _ task.TaskStatus) {
			t1.DeleteFinishedCallback(fmt.Sprintf("%s-responseCaller", h.Name()))

			select {
			case closed := <-ht.writer.(http.CloseNotifier).CloseNotify():
				if closed {
					// 499 StatusClientClosed - same as nginx
					t1.SetError(fmt.Errorf("client closed"), task.ResultRequesterGone)
					return
				}
			default:
			}

			statusCode := task.ResultCodeToHTTPCode(t1.ResultCode())

			if len(h.conf.ResponseCodeKey) != 0 {
				code, err := strconv.Atoi(
					task.ToString(t1.Value(h.conf.ResponseCodeKey), option.PluginIODataFormatLengthLimit))
				if err == nil &&
					code > 99 && code < 600 { // should seems like a valid http code, at least
					statusCode = code
				}
			}

			ht.writer.WriteHeader(statusCode)

			// TODO: Take care other headers if inputted

			if len(h.conf.ResponseBodyIOKey) != 0 {
				reader, ok := t1.Value(h.conf.ResponseBodyIOKey).(io.Reader)
				if ok {
					done := make(chan int, 1)
					reader1 := common.NewInterruptibleReader(reader)

					go func() {
						_, err := io.Copy(ht.writer, reader1)
						if err != nil {
							logger.Warnf("[load response body from reader in the task"+
								" failed, response might be incomplete: %s]", err)
						}
						done <- 0
					}()

					select {
					case <-t.Cancel():
						if h.conf.FastClose {
							reader1.Cancel()
							logger.Warnf("[load response body from reader in the task" +
								" has been cancelled, response might be incomplete]")
						} else {
							<-done
							close(done)
							reader1.Close()
						}
					case <-done:
						close(done)
						reader1.Close()
					}
				}

				closer, ok := t1.Value(h.conf.ResponseBodyIOKey).(io.Closer)
				if ok {
					err := closer.Close()
					if err != nil {
						logger.Errorf("[close response body io %s failed: %v]",
							h.conf.ResponseBodyIOKey, err)
					}
				}
			} else if len(h.conf.ResponseBodyBufferKey) != 0 {
				buff, ok := t1.Value(h.conf.ResponseBodyBufferKey).([]byte)
				if ok {
					ht.writer.Write(buff)
				}
			} else if !task.SuccessfulResult(t1.ResultCode()) && h.conf.RespondErr {
				if strings.Contains(ht.request.Header.Get("Accept-Encoding"), "gzip") {
					ht.writer.Header().Set("Content-Encoding", "gzip, deflate")
					ht.writer.Header().Set("Content-Type", "application/x-gzip")
					gz := gzip.NewWriter(ht.writer)
					gz.Write([]byte(t1.Error().Error()))
					gz.Close()
				} else {
					ht.writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
					// ascii is a subset of utf-8
					ht.writer.Write([]byte(t1.Error().Error()))
				}
			}

			ht.writer.(http.Flusher).Flush()
		}

		closeHTTPInputRequestBody := func(t1 task.Task, _ task.TaskStatus) {
			t1.DeleteFinishedCallback(fmt.Sprintf("%s-closeHTTPInputRequestBody", h.Name()))

			ht.request.Body.Close()
			close(ht.finishedChan)
		}

		logRequest := func(t1 task.Task, _ task.TaskStatus) {
			t1.DeleteFinishedCallback(fmt.Sprintf("%s-logRequest", h.Name()))

			code := t1.ResultCode()
			httpCode := task.ResultCodeToHTTPCode(code)
			// TODO: use variables(e.g. upstream_response_time_xxx) of each plugin
			// or provide a method(e.g. AddUpstreamResponseTime) of task
			// TODO: calculate real body_bytes_sent value
			logger.HTTPAccess(ht.request, httpCode, -1, t1.FinishAt().Sub(ht.receivedAt), time.Duration(-1))

			if !task.SuccessfulResult(code) {
				logger.Warnf("[http request processed unsuccessfully, "+
					"result code: %d, error: %s]", httpCode, t1.Error())
			}
		}

		shrinkWipRequestCounter := func(t1 task.Task, _ task.TaskStatus) {
			t1.DeleteFinishedCallback(fmt.Sprintf("%s-shrinkWipRequestCounter", h.Name()))

			wipReqCount, err := getHTTPInputHandlingRequestCount(ctx, h.Name())
			if err == nil {
				for !atomic.CompareAndSwapUint64(wipReqCount, *wipReqCount, *wipReqCount-1) {
				}
			}
		}

		vars, names := common.GenerateCGIEnv(ht.request)
		for k, v := range vars {
			t, err = task.WithValue(t, k, v)
			if err != nil {
				return err, task.ResultInternalServerError, t
			}
		}

		if len(h.conf.RequestHeaderNamesKey) != 0 {
			t, err = task.WithValue(t, h.conf.RequestHeaderNamesKey, names)
			if err != nil {
				return err, task.ResultInternalServerError, t
			}
		}

		if len(h.conf.RequestBodyIOKey) != 0 {
			t, err = task.WithValue(t, h.conf.RequestBodyIOKey, ht.request.Body)
			if err != nil {
				return err, task.ResultInternalServerError, t
			}
		}

		for k, v := range ht.path_params {
			t, err = task.WithValue(t, k, v)
			if err != nil {
				return err, task.ResultInternalServerError, t
			}
		}

		t.AddFinishedCallback(fmt.Sprintf("%s-responseCaller", h.Name()), respondCaller)
		t.AddFinishedCallback(fmt.Sprintf("%s-closeHTTPInputRequestBody", h.Name()), closeHTTPInputRequestBody)
		t.AddFinishedCallback(fmt.Sprintf("%s-logRequest", h.Name()), logRequest)
		t.AddFinishedCallback(fmt.Sprintf("%s-shrinkWipRequestCounter", h.Name()), shrinkWipRequestCounter)

		wipReqCount, err := getHTTPInputHandlingRequestCount(ctx, h.Name())
		if err == nil {
			atomic.AddUint64(wipReqCount, 1)
		}

		return nil, t.ResultCode(), t
	}
}

func (h *httpInput) Run(ctx pipelines.PipelineContext, t task.Task) (task.Task, error) {
	err, resultCode, t := h.receive(ctx, t)
	if err != nil {
		t.SetError(err, resultCode)
	}

	if resultCode == task.ResultTaskCancelled {
		return t, t.Error()
	} else {
		return t, nil
	}
}

func (h *httpInput) Name() string {
	return h.conf.Name
}

func (h *httpInput) Close() {
	for _, method := range h.conf.Methods {
		defaultMux.DeleteFunc(h.conf.URL, method)
	}
	if h.httpTaskChan != nil {
		close(h.httpTaskChan)
		h.httpTaskChan = nil
	}
}

////

const (
	httpInputHandlingRequestCountKey = "httpInputHandlingRequestCountKey"
)

func getHTTPInputHandlingRequestCount(ctx pipelines.PipelineContext, pluginName string) (*uint64, error) {
	bucket := ctx.DataBucket(pluginName, pipelines.DATA_BUCKET_FOR_ALL_PLUGIN_INSTANCE)
	count, err := bucket.QueryDataWithBindDefault(httpInputHandlingRequestCountKey,
		func() interface{} {
			var handlingRequestCount uint64
			return &handlingRequestCount
		})
	if err != nil {
		logger.Warnf("[BUG: query wip request counter for pipeline %s failed, "+
			"ignored to calculate wip request: %v]", ctx.PipelineName(), err)
		return nil, err
	}

	return count.(*uint64), nil
}
