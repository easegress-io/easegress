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
	urlParams    map[string]string
	finishedChan chan struct{}
}

////

type httpInputConfig struct {
	common.PluginCommonConfig
	ServerPluginName string              `json:"server_name"`
	MuxType          muxType             `json:"mux_type"`
	Scheme           string              `json:"scheme"`
	Host             string              `json:"host"`
	Port             string              `json:"port"`
	Path             string              `json:"path"`
	Query            string              `json:"query"`
	Fragment         string              `json:"fragment"`
	Priority         uint32              `json:"priority"`
	Methods          []string            `json:"methods"`
	HeadersEnum      map[string][]string `json:"headers_enum"`
	Unzip            bool                `json:"unzip"`
	RespondErr       bool                `json:"respond_error"`
	FastClose        bool                `json:"fast_close"`
	DumpRequest      string              `json:"dump_request"`

	RequestHeaderNamesKey string `json:"request_header_names_key"`
	RequestBodyIOKey      string `json:"request_body_io_key"`
	RequestHeaderKey      string `json:"request_header_key"`
	ResponseCodeKey       string `json:"response_code_key"`
	ResponseBodyIOKey     string `json:"response_body_io_key"`
	ResponseBodyBufferKey string `json:"response_body_buffer_key"`
	ResponseRemoteKey     string `json:"response_remote_key"`
	ResponseDurationKey   string `json:"response_duration_key"`
	ResponseHeaderKey     string `json:"response_header_key"`

	dumpReq bool
}

func httpInputConfigConstructor() plugins.Config {
	return &httpInputConfig{
		ServerPluginName: "httpserver-default",
		MuxType:          regexpMuxType,
		Methods:          []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodHead},
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

	if len(c.ServerPluginName) == 0 {
		return fmt.Errorf("invalid server name")
	}

	c.Scheme = ts(c.Scheme)
	c.Host = ts(c.Host)
	c.Port = ts(c.Port)
	c.Path = ts(c.Path)
	// Even in regular expression, squeezing `/v1//?` to `/v1/?` makes sense.
	c.Path = common.RemoveRepeatedRune(c.Path, '/')
	c.Query = ts(c.Query)
	c.Fragment = ts(c.Fragment)

	if len(c.Methods) == 0 {
		return fmt.Errorf("empty methods")
	}
	for i := range c.Methods {
		c.Methods[i] = ts(c.Methods[i])
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

	switch c.MuxType {
	case regexpMuxType:
		if len(c.Path) == 0 {
			return fmt.Errorf("empty path")
		}
	case paramMuxType:
		if !filepath.IsAbs(c.Path) {
			return fmt.Errorf("invalid relative url")
		}
		var positions []int
		token_booker := func(pos int, token string) (care bool, replacement string) {
			positions = append(positions, pos)
			positions = append(positions, pos+len(token)+1)
			return false, ""
		}

		_, err = common.ScanTokens(c.Path,
			false, /* do not remove escape char, due to escape char is not allowed in the path and pattern */
			token_booker)
		if err != nil {
			return err
		}

		// one and only one parameter fits in a segment of the path
		// e.g. correct: `/{a}/{b}`, wrong: `/{a}b{c}` and `/{a}b`
		for _, pos := range positions {
			if pos == 0 { // defensive, not an absolute path
				return fmt.Errorf("invalid parametric path")
			}

			if []byte(c.Path)[pos] == '{' {
				if []byte(c.Path)[pos-1] != '/' {
					return fmt.Errorf("invalid parametric path")
				}
			} else { // []byte(c.Path)[pos] == '}'
				if pos+1 < len(c.Path) && []byte(c.Path)[pos+1] != '/' {
					return fmt.Errorf("invalid parametric path")
				}
			}
		}
	default:
		return fmt.Errorf("unsupported mux type")
	}

	for key, value := range c.HeadersEnum {
		key = ts(key)
		if len(key) == 0 {
			return fmt.Errorf("invalid http headers enum")
		}
		c.HeadersEnum[key] = value
	}

	c.DumpRequest = ts(c.DumpRequest)
	if strings.ToLower(c.DumpRequest) == "auto" {
		c.dumpReq = common.StrInSlice(option.Stage, []string{"debug", "test"})
	} else if common.BoolFromStr(c.DumpRequest, false) {
		c.dumpReq = true
	} else if !common.BoolFromStr(c.DumpRequest, true) {
		c.dumpReq = false
	} else {
		return fmt.Errorf("invalid http request dump option")
	}

	c.RequestHeaderNamesKey = ts(c.RequestHeaderNamesKey)
	c.RequestBodyIOKey = ts(c.RequestBodyIOKey)
	c.RequestHeaderKey = ts(c.RequestHeaderKey)
	c.ResponseCodeKey = ts(c.ResponseCodeKey)
	c.ResponseBodyIOKey = ts(c.ResponseBodyIOKey)
	c.ResponseBodyBufferKey = ts(c.ResponseBodyBufferKey)
	c.ResponseRemoteKey = ts(c.ResponseRemoteKey)
	c.ResponseDurationKey = ts(c.ResponseDurationKey)
	c.ResponseHeaderKey = ts(c.ResponseHeaderKey)

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

func (h *httpInput) toHTTPMuxEntries() []*plugins.HTTPMuxEntry {
	var entries []*plugins.HTTPMuxEntry
	for _, method := range h.conf.Methods {
		entry := &plugins.HTTPMuxEntry{
			HTTPURLPattern: plugins.HTTPURLPattern{
				Scheme:   h.conf.Scheme,
				Host:     h.conf.Host,
				Port:     h.conf.Port,
				Path:     h.conf.Path,
				Query:    h.conf.Query,
				Fragment: h.conf.Fragment,
			},
			Method:   method,
			Priority: h.conf.Priority,
			Instance: h,
			Headers:  h.conf.HeadersEnum,
			Handler:  h.handler,
		}
		entries = append(entries, entry)
	}
	return entries
}

func (h *httpInput) Prepare(ctx pipelines.PipelineContext) {
	mux := getHTTPServerMux(ctx, h.conf.ServerPluginName, true)
	if mux != nil {
		for _, entry := range h.toHTTPMuxEntries() {
			err := mux.AddFunc(ctx.PipelineName(), entry)
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

func (h *httpInput) handler(w http.ResponseWriter, req *http.Request, urlParams map[string]string) {
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
		urlParams:    urlParams,
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

func copyHeaderFromTask(t task.Task, key string, dst http.Header) {
	respHeader, ok := t.Value(key).(http.Header)
	if ok {
		for k, v := range respHeader {
			for _, vv := range v {
				// Add appends to any existing values associated with key.
				dst.Add(k, vv)
			}
		}
	} else {
		logger.Errorf("[load header: %s in the task failed, value:%+v]", key, t.Value(key))
	}
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

	getClientReceivedCode := func(t1 task.Task) int {
		if t1.Error() != nil || len(h.conf.ResponseCodeKey) == 0 {
			return getTaskResultCode(t1)
		} else {
			return getResponseCode(t1)
		}

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

		if len(h.conf.ResponseHeaderKey) != 0 {
			copyHeaderFromTask(t1, h.conf.ResponseHeaderKey, ht.writer.Header())
		}

		ht.writer.WriteHeader(getClientReceivedCode(t1))

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
		logger.HTTPAccess(ht.request, getClientReceivedCode(t1), -1,
			t1.FinishAt().Sub(ht.receivedAt), responseDuration,
			responseRemote, getResponseCode(t1))

		if !task.SuccessfulResult(t1.ResultCode()) {
			logger.Warnf("[http request processed unsuccessfully, "+
				"result code: %d, error: %s]", getTaskResultCode(t1), t1.Error())
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

	if len(h.conf.RequestHeaderKey) != 0 {
		t, err = task.WithValue(t, h.conf.RequestHeaderKey, ht.request.Header)
		if err != nil {
			return err, task.ResultInternalServerError, t
		}
	}

	for k, v := range ht.urlParams {
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
		for _, entry := range h.toHTTPMuxEntries() {
			mux.DeleteFunc(ctx.PipelineName(), entry)
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
