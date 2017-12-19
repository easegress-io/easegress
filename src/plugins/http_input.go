package plugins

import (
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
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
	request       *http.Request
	writer        http.ResponseWriter
	routeDuration time.Duration
	receivedAt    time.Time
	urlParams     map[string]string
	finishedChan  chan struct{}
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
	c.Path = common.RemoveRepeatedByte(c.Path, '/')
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
	waitQueueLengthIndicatorAdded bool
	wipRequestCountIndicatorAdded bool
	contexts                      *sync.Map
}

func httpInputConstructor(conf plugins.Config) (plugins.Plugin, plugins.PluginType, bool, error) {
	c, ok := conf.(*httpInputConfig)
	if !ok {
		return nil, plugins.SourcePlugin, false, fmt.Errorf(
			"config type want *HTTPInputConfig got %T", conf)
	}

	h := &httpInput{
		conf:         c,
		httpTaskChan: make(chan *httpTask, 32767),
		contexts:     new(sync.Map),
	}

	h.instanceId = fmt.Sprintf("%p", h)

	return h, plugins.SourcePlugin, false, nil
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
			return h.getHTTPTaskQueueLength(), nil
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
			return atomic.LoadInt64(wipReqCount), nil
		})
	if err != nil {
		logger.Warnf("[BUG: register plugin %s indicator %s failed, "+
			"ignored to expose customized statistics indicator: %s]", h.Name(), "WIP_REQUEST_COUNT", err)
	}

	h.wipRequestCountIndicatorAdded = added

	h.contexts.Store(ctx.PipelineName(), ctx)
}

func (h *httpInput) handler(w http.ResponseWriter, req *http.Request, urlParams map[string]string,
	routeDuration time.Duration) {

	if req.ContentLength >= 0 { // content length is known
		reader := io.LimitReader(req.Body, req.ContentLength)
		req.Body = common.IOReaderToReaderCloser(reader, req.Body)
	}

	if h.conf.Unzip && strings.Contains(req.Header.Get("Content-Encoding"), "gzip") {
		var err error
		req.Body, err = gzip.NewReader(req.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	}

	httpTask := httpTask{
		request:       req,
		writer:        w,
		routeDuration: routeDuration,
		receivedAt:    common.Now(),
		urlParams:     urlParams,
		finishedChan:  make(chan struct{}),
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

		h.contexts.Range(func(key, value interface{}) bool {
			value.(pipelines.PipelineContext).TriggerSourceInput(
				"httpTaskQueueLengthGetter", h.getHTTPTaskQueueLength)
			return true // iterate next
		})
	}()

	<-httpTask.finishedChan
}

func (h *httpInput) getHTTPTaskQueueLength() uint32 {
	return uint32(len(h.httpTaskChan))
}

func (h *httpInput) receive(ctx pipelines.PipelineContext, t task.Task) (error, task.TaskResultCode) {
	var ok bool
	var ht *httpTask
	var err error

	notifier := getHTTPServerGoneNotifier(ctx, h.conf.ServerPluginName, false)
	if notifier == nil {
		return fmt.Errorf("http server %s gone", h.conf.ServerPluginName), task.ResultServerGone
	}

	select {
	case ht, ok = <-h.httpTaskChan:
		if !ok {
			return fmt.Errorf("plugin %s has been closed", h.Name()),
				task.ResultInternalServerError
		}
	case <-t.Cancel():
		return fmt.Errorf("task is cancelled by %s", t.CancelCause()), task.ResultTaskCancelled
	case <-notifier:
		return fmt.Errorf("http server %s gone", h.conf.ServerPluginName), task.ResultServerGone
	}

	if h.conf.dumpReq {
		logger.HTTPReqDump(ctx.PipelineName(), h.Name(), h.instanceId, t.StartAt().UnixNano(), ht.request)
	}

	vars, names := common.GenerateCGIEnv(ht.request)
	for k, v := range vars {
		t.WithValue(k, v)
	}

	if len(h.conf.RequestHeaderNamesKey) != 0 {
		t.WithValue(h.conf.RequestHeaderNamesKey, names)
	}

	body := common.NewTimeReader(ht.request.Body)
	if len(h.conf.RequestBodyIOKey) != 0 {
		t.WithValue(h.conf.RequestBodyIOKey, body)
	}

	if len(h.conf.RequestHeaderKey) != 0 {
		t.WithValue(h.conf.RequestHeaderKey, ht.request.Header)
	}

	for k, v := range ht.urlParams {
		t.WithValue(k, v)
	}

	respondCallerAndLogRequest := func(t1 task.Task, _ task.TaskStatus) {
		defer h.closeResponseBody(t1)

		select {
		case closed := <-ht.writer.(http.CloseNotifier).CloseNotify():
			if closed {
				// 499 StatusClientClosed - same as nginx
				t1.SetError(fmt.Errorf("client gone"), task.ResultRequesterGone)
				return
			}
		default:
		}

		if len(h.conf.ResponseHeaderKey) != 0 {
			copyHeaderFromTask(t1, h.conf.ResponseHeaderKey, ht.writer.Header())
		}

		ht.writer.WriteHeader(getClientReceivedCode(t1, h.conf.ResponseCodeKey))

		// TODO: Take care other headers if inputted

		var bodyBytesSent int64 = -1 // -1 indicates we can't provide a proper value
		var readRespBodyElapse, writeClientBodyElapse time.Duration
		// Use customized ResponseBodyBuffer first
		if len(h.conf.ResponseBodyBufferKey) != 0 && t1.Value(h.conf.ResponseBodyBufferKey) != nil {
			buf := task.ToBytes(t1.Value(h.conf.ResponseBodyBufferKey), option.PluginIODataFormatLengthLimit)
			bodyBytesSent = int64(len(buf))

			writeStartAt := time.Now()
			ht.writer.Write(buf)
			writeClientBodyElapse = time.Since(writeStartAt)
		} else if len(h.conf.ResponseBodyIOKey) != 0 {
			reader, ok := t1.Value(h.conf.ResponseBodyIOKey).(io.Reader)
			if ok {
				done := make(chan int, 1)
				ir := common.NewInterruptibleReader(reader)
				iw := common.NewInterruptibleWriter(ht.writer)
				tr := common.NewTimeReader(ir)
				tw := common.NewTimeWriter(iw)

				go func() {
					written, err := io.Copy(tw, tr)
					if err != nil {
						logger.Warnf("[read or write body failed, "+
							"response might be incomplete: %s]", err)
					} else {
						bodyBytesSent = written
					}
					done <- 0
				}()

				select {
				case closed := <-ht.writer.(http.CloseNotifier).CloseNotify():
					if closed {
						err := fmt.Errorf("client gone")
						iw.Cancel(err)
						ir.Close()
						<-done

						// 499 StatusClientClosed - same as nginx
						t1.SetError(err, task.ResultRequesterGone)
					}
				case <-t1.Cancel():
					if h.conf.FastClose {
						ir.Cancel(t1.CancelCause())
						iw.Close()
						<-done

						logger.Warnf("[load response body from reader in the task" +
							" has been cancelled, response might be incomplete]")
					} else {
						<-done
						ir.Close()
						iw.Close()
					}
				case <-done:
					ir.Close()
					iw.Close()
				}

				close(done)

				readRespBodyElapse = tr.Elapse()
				writeClientBodyElapse = tw.Elapse()
			}
		} else if !task.SuccessfulResult(t1.ResultCode()) && h.conf.RespondErr {
			if strings.Contains(ht.request.Header.Get("Accept-Encoding"), "gzip") {
				ht.writer.Header().Set("Content-Encoding", "gzip, deflate")
				ht.writer.Header().Set("Content-Type", "application/x-gzip")
				gz := gzip.NewWriter(ht.writer)
				bytes := []byte(t1.Error().Error())
				written, _ := gz.Write(bytes) // ignore error
				bodyBytesSent = int64(written)
				gz.Close()
			} else {
				ht.writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
				// ascii is a subset of utf-8
				bytes := []byte(t1.Error().Error())
				bodyBytesSent = int64(len(bytes))

				writeStartAt := time.Now()
				ht.writer.Write(bytes)
				writeClientBodyElapse = time.Since(writeStartAt)
			}
		}

		logRequest(ht, t1, h.conf.ResponseCodeKey, h.conf.ResponseRemoteKey,
			h.conf.ResponseDurationKey, readRespBodyElapse, writeClientBodyElapse, body.Elapse(),
			bodyBytesSent, ht.routeDuration)
	}

	closeHTTPInputRequestBody := func(t1 task.Task, _ task.TaskStatus) {
		ht.request.Body.Close()
		close(ht.finishedChan)
	}

	shrinkWipRequestCounter := func(t1 task.Task, _ task.TaskStatus) {
		wipReqCount, err := getHTTPInputHandlingRequestCount(ctx, h.Name())
		if err == nil {
			atomic.AddInt64(wipReqCount, -1)
		}
	}

	t.AddFinishedCallback(fmt.Sprintf("%s-responseCallerAndLogRequest", h.Name()), respondCallerAndLogRequest)
	t.AddFinishedCallback(fmt.Sprintf("%s-closeHTTPInputRequestBody", h.Name()), closeHTTPInputRequestBody)
	t.AddFinishedCallback(fmt.Sprintf("%s-shrinkWipRequestCounter", h.Name()), shrinkWipRequestCounter)

	wipReqCount, err := getHTTPInputHandlingRequestCount(ctx, h.Name())
	if err == nil {
		atomic.AddInt64(wipReqCount, 1)
	}

	return nil, t.ResultCode()
}

func (h *httpInput) Run(ctx pipelines.PipelineContext, t task.Task) error {
	err, resultCode := h.receive(ctx, t)
	if err != nil {
		t.SetError(err, resultCode)
	}

	if resultCode == task.ResultTaskCancelled {
		return t.Error()
	} else {
		return nil
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

	h.contexts.Delete(ctx.PipelineName())
}

func (h *httpInput) Close() {
	if h.httpTaskChan != nil {
		close(h.httpTaskChan)
		h.httpTaskChan = nil
	}
}

////

const (
	httpInputHandlingRequestCountKey = "httpInputHandlingRequestCountKey"
)

func getHTTPInputHandlingRequestCount(ctx pipelines.PipelineContext, pluginName string) (*int64, error) {
	bucket := ctx.DataBucket(pluginName, pipelines.DATA_BUCKET_FOR_ALL_PLUGIN_INSTANCE)
	count, err := bucket.QueryDataWithBindDefault(httpInputHandlingRequestCountKey,
		func() interface{} {
			var handlingRequestCount int64
			return &handlingRequestCount
		})
	if err != nil {
		logger.Warnf("[BUG: query wip request counter for pipeline %s failed, "+
			"ignored to calculate wip request: %v]", ctx.PipelineName(), err)
		return nil, err
	}

	return count.(*int64), nil
}

////

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

func getResponseCode(t task.Task, responseCodeKey string) int {
	statusCode := task.ResultCodeToHTTPCode(t.ResultCode())
	if len(responseCodeKey) != 0 {
		code, err := strconv.Atoi(
			task.ToString(t.Value(responseCodeKey), option.PluginIODataFormatLengthLimit))
		if err == nil {
			statusCode = code
		}
	}
	return statusCode
}

func getClientReceivedCode(t task.Task, responseCodeKey string) int {
	if t.Error() != nil || len(responseCodeKey) == 0 {
		return task.ResultCodeToHTTPCode(t.ResultCode())
	} else {
		return getResponseCode(t, responseCodeKey)
	}
}

func logRequest(ht *httpTask, t task.Task, responseCodeKey, responseRemoteKey,
	responseDurationKey string, readRespBodyElapse, writeClientBodyElapse, readClientBodyElapse time.Duration,
	bodyBytesSent int64, routeDuration time.Duration) {

	var responseRemote = ""
	value := t.Value(responseRemoteKey)
	if value != nil {
		rr, ok := value.(string)
		if ok {
			responseRemote = rr
		}
	}

	responseDuration := readRespBodyElapse
	value = nil
	value = t.Value(responseDurationKey)
	if value != nil {
		rd, ok := value.(time.Duration)
		if ok {
			responseDuration += rd
		}
	}

	requestTime := common.Since(ht.receivedAt) + routeDuration

	// TODO: use variables(e.g. upstream_response_time_xxx) of each plugin
	// or provide a method(e.g. AddUpstreamResponseTime) of task
	logger.HTTPAccess(ht.request, getClientReceivedCode(t, responseCodeKey), bodyBytesSent,
		requestTime, responseDuration, responseRemote,
		getResponseCode(t, responseCodeKey), writeClientBodyElapse, readClientBodyElapse,
		routeDuration)

	if !task.SuccessfulResult(t.ResultCode()) {
		logger.Warnf("[http request processed unsuccessfully, "+
			"result code: %d, error: %s]", task.ResultCodeToHTTPCode(t.ResultCode()), t.Error())
	}
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
		// There are some normal cases that the header key is nil in task
		// Because header key producer don't write them
		logger.Debugf("[load header: %s in the task failed, value:%+v]", key, t.Value(key))
	}
}
