package plugins

import (
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
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

type httpTask struct {
	request      *http.Request
	writer       http.ResponseWriter
	receivedAt   time.Time
	path_params  map[string]string
	finishedChan chan struct{}
}

////

type httpInputConfig struct {
	common.PluginCommonConfig
	ServerPluginName string              `json:"server_name"`
	URL              string              `json:"url"`
	Methods          []string            `json:"methods"`
	HeadersEnum      map[string][]string `json:"headers_enum"`
	Unzip            bool                `json:"unzip"`
	RespondErr       bool                `json:"respond_error"`
	FastClose        bool                `json:"fast_close"`
	DumpRequest      string              `json:"dump_request"`

	RequestHeaderNamesKey string `json:"request_header_names_key"`
	RequestBodyIOKey      string `json:"request_body_io_key"`
	ResponseCodeKey       string `json:"response_code_key"`
	ResponseBodyIOKey     string `json:"response_body_io_key"`
	ResponseBodyBufferKey string `json:"response_body_buffer_key"`
	ResponseRemoteKey     string `json:"response_remote_key"`
	ResponseDurationKey   string `json:"response_duration_key"`

	dumpReq bool
}

func httpInputConfigConstructor() plugins.Config {
	return &httpInputConfig{
		ServerPluginName: "httpserver-default",
		Methods:          []string{http.MethodGet},
		Unzip:            true,
		DumpRequest:      "auto",
	}
}

func (c *httpInputConfig) Prepare(pipelineNames []string) error {
	err := c.PluginCommonConfig.Prepare(pipelineNames)
	if err != nil {
		return err
	}

	ts := strings.TrimSpace
	c.ServerPluginName = ts(c.ServerPluginName)
	c.URL = ts(c.URL)

	if len(c.ServerPluginName) == 0 {
		return fmt.Errorf("invalid server name")
	}

	for i := range c.Methods {
		c.Methods[i] = ts(c.Methods[i])
	}

	c.DumpRequest = ts(c.DumpRequest)
	c.RequestHeaderNamesKey = ts(c.RequestHeaderNamesKey)
	c.RequestBodyIOKey = ts(c.RequestBodyIOKey)
	c.ResponseCodeKey = ts(c.ResponseCodeKey)
	c.ResponseBodyIOKey = ts(c.ResponseBodyIOKey)
	c.ResponseBodyBufferKey = ts(c.ResponseBodyBufferKey)
	c.ResponseRemoteKey = ts(c.ResponseRemoteKey)
	c.ResponseDurationKey = ts(c.ResponseDurationKey)

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

type httpInput struct {
	conf                          *httpInputConfig
	httpTaskChan                  chan *httpTask
	instanceId                    string
	queueLength                   uint64
	waitQueueLengthIndicatorAdded bool
	wipRequestCountIndicatorAdded bool
}

func httpInputConstructor(conf plugins.Config) (plugins.Plugin, error) {
	c, ok := conf.(*httpInputConfig)
	if !ok {
		return nil, fmt.Errorf("config type want *HTTPInputConfig got %T", conf)
	}

	h := &httpInput{
		conf:         c,
		httpTaskChan: make(chan *httpTask, 32767),
	}

	h.instanceId = fmt.Sprintf("%p", h)

	return h, nil
}

func (h *httpInput) Prepare(ctx pipelines.PipelineContext) {
	mux := getHTTPServerMux(ctx, h.conf.ServerPluginName, true)
	if mux != nil {
		for _, method := range h.conf.Methods {
			err := mux.AddFunc(ctx.PipelineName(), h.conf.URL, method, h.conf.HeadersEnum, h.handler)
			if err != nil {
				logger.Errorf("[add handler to server %s failed: %v]", h.conf.ServerPluginName, err)
			}
		}
	}

	added, err := ctx.Statistics().RegisterPluginIndicator(h.Name(), h.instanceId, "WAIT_QUEUE_LENGTH",
		"The length of wait queue which contains requests wait to be handled by a pipeline.",
		func(pluginName, indicatorName string) (interface{}, error) {
			return h.queueLength, nil
		})
	if err != nil {
		logger.Warnf("[BUG: register plugin %s indicator %s failed, "+
			"ignored to expose customized statistics indicator: %s]", h.Name(), "WAIT_QUEUE_LENGTH", err)
	}

	h.waitQueueLengthIndicatorAdded = added

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
	}

	h.wipRequestCountIndicatorAdded = added
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
	var ok bool
	var ht *httpTask
	var err error

	notifier := getHTTPServerGoneNotifier(ctx, h.conf.ServerPluginName, false)
	if notifier == nil {
		return fmt.Errorf("http server %s gone", h.conf.ServerPluginName), task.ResultServerGone, t
	}

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
	case <-notifier:
		return fmt.Errorf("http server %s gone", h.conf.ServerPluginName), task.ResultServerGone, t
	}

	if h.conf.dumpReq {
		logger.HTTPReqDump(ctx.PipelineName(), h.Name(), h.instanceId, t.StartAt().UnixNano(), ht.request)
	}

	getTaskResultCode := func(t1 task.Task) int {
		return task.ResultCodeToHTTPCode(t1.ResultCode())
	}

	getResponseCode := func(t1 task.Task) int {
		statusCode := getTaskResultCode(t1)
		if len(h.conf.ResponseCodeKey) != 0 {
			code, err := strconv.Atoi(
				task.ToString(t1.Value(h.conf.ResponseCodeKey), option.PluginIODataFormatLengthLimit))
			if err == nil {
				statusCode = code
			}
		}
		return statusCode
	}

	respondCaller := func(t1 task.Task, _ task.TaskStatus) {
		t1.DeleteFinishedCallback(fmt.Sprintf("%s-responseCaller", h.Name()))

		defer h.closeResponseBody(t1)

		select {
		case closed := <-ht.writer.(http.CloseNotifier).CloseNotify():
			if closed {
				// 499 StatusClientClosed - same as nginx
				t1.SetError(fmt.Errorf("client closed"), task.ResultRequesterGone)
				return
			}
		default:
		}

		taskResultCode := getTaskResultCode(t1)
		ht.writer.WriteHeader(taskResultCode)

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
				case <-t1.Cancel():
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

		var responseRemote string = ""
		value := t1.Value(h.conf.ResponseRemoteKey)
		if value != nil {
			rr, ok := value.(string)
			if ok {
				responseRemote = rr
			}
		}

		taskResultCode := getTaskResultCode(t1)
		responseCode := getResponseCode(t1)

		var responseDuration time.Duration = 0
		value = nil
		value = t1.Value(h.conf.ResponseDurationKey)
		if value != nil {
			rd, ok := value.(time.Duration)
			if ok {
				responseDuration = rd
			}
		}

		// TODO: use variables(e.g. upstream_response_time_xxx) of each plugin
		// or provide a method(e.g. AddUpstreamResponseTime) of task
		// TODO: calculate real body_bytes_sent value, which need read data from repondCaller.
		logger.HTTPAccess(ht.request, taskResultCode, -1,
			t1.FinishAt().Sub(ht.receivedAt), responseDuration,
			responseRemote, responseCode)

		if !task.SuccessfulResult(t1.ResultCode()) {
			logger.Warnf("[http request processed unsuccessfully, "+
				"result code: %d, error: %s]", taskResultCode, t1.Error())
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

func (h *httpInput) CleanUp(ctx pipelines.PipelineContext) {
	mux := getHTTPServerMux(ctx, h.conf.ServerPluginName, false)
	if mux != nil {
		for _, method := range h.conf.Methods {
			mux.DeleteFunc(ctx.PipelineName(), h.conf.URL, method)
		}
	}

	if h.waitQueueLengthIndicatorAdded {
		ctx.Statistics().UnregisterPluginIndicator(h.Name(), h.instanceId, "WAIT_QUEUE_LENGTH")
	}

	if h.wipRequestCountIndicatorAdded {
		ctx.Statistics().UnregisterPluginIndicator(h.Name(), h.instanceId, "WIP_REQUEST_COUNT")
	}
}

func (h *httpInput) Close() {
	if h.httpTaskChan != nil {
		close(h.httpTaskChan)
		h.httpTaskChan = nil
	}
}

func (h *httpInput) closeResponseBody(t task.Task) {
	if len(h.conf.ResponseBodyIOKey) == 0 {
		return
	}

	closer, ok := t.Value(h.conf.ResponseBodyIOKey).(io.Closer)
	if ok {
		err := closer.Close()
		if err != nil {
			logger.Errorf("[close response body io %s failed: %v]",
				h.conf.ResponseBodyIOKey, err)
		}
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
