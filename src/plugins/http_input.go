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

	"common"
	"logger"
	"pipelines"
	"task"
)

// TODO: Moves this into plugin, aligns https server life-cycle with plugin life-cycle
func init() {
	crt_name := common.Host + "-cert.pem"
	key_name := common.Host + "-key.pem"
	SSL_CRT_PATH := filepath.Join(common.CERT_HOME_DIR, crt_name)
	SSL_KEY_PATH := filepath.Join(common.CERT_HOME_DIR, key_name)

	logger.Infof("[cert file: %s]", SSL_CRT_PATH)
	logger.Infof("[key file: %s]", SSL_KEY_PATH)

	go func() {
		tls := true
		if _, err := os.Stat(SSL_CRT_PATH); os.IsNotExist(err) {
			logger.Warnf("[cert file %s not found]", SSL_CRT_PATH)
			tls = false
		}

		if _, err := os.Stat(SSL_KEY_PATH); os.IsNotExist(err) {
			logger.Warnf("[key file %s not found]", SSL_KEY_PATH)
			tls = false
		}

		var (
			err    error
			server string
		)
		if tls {
			err = http.ListenAndServeTLS("0.0.0.0:10443", SSL_CRT_PATH, SSL_KEY_PATH, defaultMux)
			server = "HTTPS"
		} else {
			err = http.ListenAndServe("0.0.0.0:10080", defaultMux)
			server = "HTTP"
		}

		if err != nil {
			logger.Errorf("Start %s server failed:", server, err)
		}
	}()
}

////

type HTTPInputConfig struct {
	CommonConfig
	URL         string              `json:"url"`
	Method      string              `json:"method"`
	HeadersEnum map[string][]string `json:"headers_enum"`
	Unzip       bool                `json:"unzip"`
	RespondErr  bool                `json:"respond_error"`

	RequestBodyIOKey      string `json:"request_body_io_key"`
	ResponseCodeKey       string `json:"response_code_key"`
	ResponseBodyIOKey     string `json:"response_body_io_key"`
	ResponseBodyBufferKey string `json:"response_body_buffer_key"`
}

func HTTPInputConfigConstructor() Config {
	return &HTTPInputConfig{
		Method: http.MethodGet,
		Unzip:  true,
	}
}

func (c *HTTPInputConfig) Prepare() error {
	err := c.CommonConfig.Prepare()
	if err != nil {
		return err
	}

	ts := strings.TrimSpace
	c.URL = ts(c.URL)
	c.Method = ts(c.Method)

	c.RequestBodyIOKey = ts(c.RequestBodyIOKey)
	c.ResponseCodeKey = ts(c.ResponseCodeKey)
	c.ResponseBodyIOKey = ts(c.ResponseBodyIOKey)
	c.ResponseBodyBufferKey = ts(c.ResponseBodyBufferKey)

	if !filepath.IsAbs(c.URL) {
		return fmt.Errorf("invalid absolutate url")
	}

	_, ok := supportedMethods[c.Method]
	if !ok {
		return fmt.Errorf("invalid http method")
	}

	for key, value := range c.HeadersEnum {
		key = ts(key)
		if len(key) == 0 {
			return fmt.Errorf("invalid http headers enum")
		}
		c.HeadersEnum[key] = value
	}

	return nil
}

////

type httpTask struct {
	request      *http.Request
	writer       http.ResponseWriter
	finishedChan chan struct{}
}

////
type httpInput struct {
	conf         *HTTPInputConfig
	httpTaskChan chan *httpTask
	instanceId   string
	queueLength  uint64
}

func HTTPInputConstructor(conf Config) (Plugin, error) {
	c, ok := conf.(*HTTPInputConfig)
	if !ok {
		return nil, fmt.Errorf("config type want *HTTPInputConfig got %T", conf)
	}

	h := &httpInput{
		conf:         c,
		httpTaskChan: make(chan *httpTask, 32767),
	}

	h.instanceId = fmt.Sprintf("%p", h)

	err := defaultMux.HandleFunc(h.conf.URL, h.conf.Method, h.conf.HeadersEnum, h.handler)
	if err != nil {
		return nil, fmt.Errorf("handle failed: %v", err)
	}

	return h, err
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
			wipReqCount, err := getHttpInputHandlingRequestCount(ctx, pluginName)
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

func (h *httpInput) handler(w http.ResponseWriter, req *http.Request) {
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

		responseCaller := func(t1 task.Task, _ task.TaskStatus) {
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

			statusCode := task.ResultCodeToHttpCode(t1.ResultCode())

			if len(h.conf.ResponseCodeKey) != 0 {
				code, err := strconv.Atoi(task.ToString(t1.Value(h.conf.ResponseCodeKey)))
				if err == nil &&
					code > 99 && code < 600 { // should seems like a valid http code, at least
					statusCode = code
				}
			}

			ht.writer.WriteHeader(statusCode)

			// TODO: Take care other headers if inputted

			if len(h.conf.ResponseBodyIOKey) != 0 {
				reader, ok := t1.Value(h.conf.ResponseBodyIOKey).(io.ReadCloser)
				if ok {
					_, err := io.Copy(ht.writer, reader)
					if err != nil {
						logger.Warnf("[load response body from reader in the task failed, "+
							"response might be incomplete: %s]", err)
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

		closeHttpInputRequestBody := func(t1 task.Task, _ task.TaskStatus) {
			t1.DeleteFinishedCallback(fmt.Sprintf("%s-closeHttpInputRequestBody", h.Name()))

			ht.request.Body.Close()
			close(ht.finishedChan)
		}

		logRequest := func(t1 task.Task, _ task.TaskStatus) {
			t1.DeleteFinishedCallback(fmt.Sprintf("%s-logRequest", h.Name()))

			code := t1.ResultCode()
			httpCode := task.ResultCodeToHttpCode(code)
			// TODO: use variables(e.g. upstream_response_time_xxx) of each plugin
			// or provide a method(e.g. AddUpstreamResponseTime) of task
			logger.HTTPAccess(ht.request, httpCode, 0, t1.FinishAt().Sub(t1.StartAt()), time.Duration(0))

			if !task.SuccessfulResult(code) {
				logger.Warnf("[http request processed unsuccesfully, "+
					"result code: %d, error: %s]", httpCode, t1.Error())
			}
		}

		shrinkWipRequestCounter := func(t1 task.Task, _ task.TaskStatus) {
			t1.DeleteFinishedCallback(fmt.Sprintf("%s-shrinkWipRequestCounter", h.Name()))

			wipReqCount, err := getHttpInputHandlingRequestCount(ctx, h.Name())
			if err == nil {
				for !atomic.CompareAndSwapUint64(wipReqCount, *wipReqCount, *wipReqCount-1) {
				}
			}
		}

		for k, v := range common.GenerateCGIEnv(ht.request) {
			t, err = task.WithValue(t, k, v)
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

		t.AddFinishedCallback(fmt.Sprintf("%s-responseCaller", h.Name()), responseCaller)
		t.AddFinishedCallback(fmt.Sprintf("%s-closeHttpInputRequestBody", h.Name()), closeHttpInputRequestBody)
		t.AddFinishedCallback(fmt.Sprintf("%s-logRequest", h.Name()), logRequest)
		t.AddFinishedCallback(fmt.Sprintf("%s-shrinkWipRequestCounter", h.Name()), shrinkWipRequestCounter)

		wipReqCount, err := getHttpInputHandlingRequestCount(ctx, h.Name())
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
	defaultMux.DeleteFunc(h.conf.URL, h.conf.Method)
	if h.httpTaskChan != nil {
		close(h.httpTaskChan)
		h.httpTaskChan = nil
	}
}

////

const (
	httpInputHandlingRequestCountKey = "httpInputHandlingRequestCountKey"
)

func getHttpInputHandlingRequestCount(ctx pipelines.PipelineContext, pluginName string) (*uint64, error) {
	bucket := ctx.DataBucket(pluginName, pipelines.DATA_BUCKET_FOR_ALL_PLUGIN_INSTANCE)
	count, err := bucket.QueryDataWithBindDefault(httpInputHandlingRequestCountKey,
		func() interface{} {
			var handlingRequestCount uint64
			return &handlingRequestCount
		})
	if err != nil {
		logger.Warnf("[BUG: query wip request counter for pipeline %s failed, "+
			"ignored to calculate wip request: %s]", ctx.PipelineName(), err)
		return nil, err
	}

	return count.(*uint64), nil
}
