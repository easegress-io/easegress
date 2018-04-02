package plugins_test

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"common"
	"config"
	"engine"
	eghttp "http"
	"logger"
	"model"
	"plugins"
)

// TIPS: use `-log` to specify the log home dir
// go test	plugins_test -log=/Users/shengdong/easegateway/src/plugins_test -run TestHTTPPRoxy_NetHTTPServeSingleTrunkedResponse
const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func init() {
	rand.Seed(time.Now().UnixNano())
}

func randBytes() []byte {
	n := rand.Int31() % 10240 // maximum 10k
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Int63()%int64(len(letterBytes))]
	}
	return b
}

func getAllPlugins(pluginSpecs []config.PluginSpec, t *testing.T) []*config.PluginSpec {
	var ret []*config.PluginSpec

	for _, spec := range pluginSpecs {
		constructor, e := plugins.GetConstructor(spec.Type)
		if e != nil {
			t.Fatalf("get plugin constructor failed: %v", e)
		}
		spec.Constructor = constructor
		s := spec // duplicate one
		ret = append(ret, &s)
	}
	return ret
}

func createPipeline(rawPluginSpecs []config.PluginSpec, pipelineSpecs []*config.PipelineSpec, t *testing.T) (*model.Model, engine.PipelineScheduler) {
	pluginSpecs := getAllPlugins(rawPluginSpecs, t)
	m := model.NewModel() // stat registry will add lifecycle callback on model
	var scheduler engine.PipelineScheduler
	launchPipeline := func(newPipeline *model.Pipeline) {
		statistics := m.StatRegistry().GetPipelineStatistics(newPipeline.Name())
		if statistics == nil {
			t.Fatalf("[launch pipeline %s failed: pipeline statistics not found]", newPipeline.Name())
			return
		}

		scheduler = engine.CreatePipelineScheduler(newPipeline)
		ctx := m.CreatePipelineContext(newPipeline.Config(), statistics, nil /* we don't need SourceInputTrigger */)
		scheduler.Start(ctx, statistics, m)
		logger.Infof("scheduler started")
	}

	m.AddPipelineAddedCallback("launchPipeline", launchPipeline,
		common.NORMAL_PRIORITY_CALLBACK)

	if err := m.LoadPlugins(pluginSpecs); err != nil {
		t.Fatalf("load plugins failed: %v", err)
	}

	if err := m.LoadPipelines(pipelineSpecs); err != nil {
		t.Fatalf("load pipelines failed: %v", err)
	}
	return m, scheduler
}

// use "localhost", to keep different with backend server host(127.0.0.1)
var defaultHTTPProxyHost = "localhost"

// use 8080 to avoid sudo when using 80
var defaultHTTPProxyPort = uint16(8080)
var defaultRequestBodyKey = "REQUEST_BODY"
var defaultResponseBodyKey = "RESPONSE_BODY"
var defaultResponseHeaderKey = "RESPONSE_HEADER"
var defaultRequestHeaderKey = "REQUEST_HEADER"

// generate eg's pipeline and plugin as a http proxy api gateway
func getHTTPProxyCfg(httpImpl httpImpl, proxyPath string, backendUrl, backendMethod string) ([]config.PluginSpec, []*config.PipelineSpec) {
	pipelineCfg := common.PipelineCommonConfig{
		Name:                              "http_proxy",
		Plugins:                           []string{"httpServer", "httpInput", "httpOutput"},
		ParallelismCount:                  1,
		CrossPipelineRequestBacklogLength: 512,
	}

	var httpSrvCfg *plugins.HTTPServerConfig
	httpSrvCfg = plugins.HTTPServerConfigConstructor().(*plugins.HTTPServerConfig)
	httpSrvCfg.Name = "httpServer"
	httpSrvCfg.Host = defaultHTTPProxyHost
	httpSrvCfg.Port = defaultHTTPProxyPort
	httpSrvCfg.Type = httpImpl.server

	var httpInputCfg *plugins.HTTPInputConfig
	httpInputCfg = plugins.HTTPInputConfigConstructor().(*plugins.HTTPInputConfig)
	httpInputCfg.Name = "httpInput"
	httpInputCfg.Path = proxyPath
	httpInputCfg.Unzip = false
	httpInputCfg.ServerPluginName = httpSrvCfg.Name
	httpInputCfg.RequestBodyIOKey = defaultRequestBodyKey
	httpInputCfg.ResponseBodyIOKey = defaultResponseBodyKey
	httpInputCfg.ResponseHeaderKey = defaultResponseHeaderKey
	httpInputCfg.RequestHeaderKey = defaultRequestHeaderKey

	var httpOutputCfg *plugins.HTTPOutputConfig
	httpOutputCfg = plugins.HTTPOutputConfigConstructor().(*plugins.HTTPOutputConfig)
	httpOutputCfg.Name = "httpOutput"
	httpOutputCfg.URLPattern = backendUrl
	httpOutputCfg.HeaderPatterns = make(map[string]string)
	httpOutputCfg.HeaderPatterns["Host"] = "{SERVER_NAME}"
	httpOutputCfg.Type = httpImpl.output
	httpOutputCfg.Method = backendMethod
	httpOutputCfg.RequestBodyIOKey = defaultRequestBodyKey
	httpOutputCfg.ResponseBodyIOKey = defaultResponseBodyKey
	httpOutputCfg.ResponseHeaderKey = defaultResponseHeaderKey
	httpOutputCfg.RequestHeaderKey = defaultRequestHeaderKey
	httpOutputCfg.DumpRequest = "y"

	pipelines := []*config.PipelineSpec{
		&config.PipelineSpec{
			Type:   "LinearPipeline",
			Config: pipelineCfg,
		},
	}
	plugins := []config.PluginSpec{
		config.PluginSpec{
			Type:   "HTTPServer",
			Config: httpSrvCfg,
		},
		config.PluginSpec{
			Type:   "HTTPInput",
			Config: httpInputCfg,
		},
		config.PluginSpec{
			Type:   "HTTPOutput",
			Config: httpOutputCfg,
		},
	}

	return plugins, pipelines
}

type mux struct {
	Handler http.HandlerFunc
}

func (m *mux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.Handler(w, r)
}

// CreateBackendServer starts and returns a new httptest.Server listening on localhost.
// The caller should call Close when finished, to shut it down.
func createBackendServer(handler http.HandlerFunc) *httptest.Server {
	mux := new(mux)
	mux.Handler = handler
	// listens on 127.0.0.1:randomport
	ts := httptest.NewServer(mux)
	return ts
}

func gzipBytes(body []byte, t *testing.T) []byte {
	gziped := bytes.NewBuffer(nil)
	gzipWriter := gzip.NewWriter(gziped)
	_, err := gzipWriter.Write(body)
	if err != nil {
		t.Fatalf("failed to gzip content: %s, err: %v", body, err)
	}
	gzipWriter.Close()
	return gziped.Bytes()
}

//// Test single get request
func TestHTTPProxy_NetHTTPServeSingleGetRequest(t *testing.T) {
	httpImpl := httpImpl{eghttp.NetHTTPStr, eghttp.NetHTTPStr}
	request := createRequest(false /*un gziped*/, http.MethodGet, t)
	response := createResponse(false /*un gziped*/, false /* un trunked */, t)
	testHTTPServeSingleRequest(httpImpl, request, response, t)
}

func TestHTTPProxy_FastHTTPServeSingleGetRequest(t *testing.T) {
	httpImpl := httpImpl{eghttp.FastHTTPStr, eghttp.FastHTTPStr}
	request := createRequest(false /*un gziped*/, http.MethodGet, t)
	response := createResponse(false /*un gziped*/, false /* un trunked */, t)
	testHTTPServeSingleRequest(httpImpl, request, response, t)
}

func TestHTTPProxy_MixNetFastHTTPServeSingleGetRequest(t *testing.T) {
	httpImpl := httpImpl{eghttp.NetHTTPStr, eghttp.FastHTTPStr}
	request := createRequest(false /*un gziped*/, http.MethodGet, t)
	response := createResponse(false /*un gziped*/, false /* un trunked */, t)
	testHTTPServeSingleRequest(httpImpl, request, response, t)
}

func TestHTTPProxy_MixFastNetHTTPServeSingleGetRequest(t *testing.T) {
	httpImpl := httpImpl{eghttp.FastHTTPStr, eghttp.NetHTTPStr}
	request := createRequest(false /*un gziped*/, http.MethodGet, t)
	response := createResponse(false /*un gziped*/, false /* un trunked */, t)
	testHTTPServeSingleRequest(httpImpl, request, response, t)
}

//// Test single post request
func TestHTTPProxy_NetHTTPServeSinglePostRequest(t *testing.T) {
	httpImpl := httpImpl{eghttp.NetHTTPStr, eghttp.NetHTTPStr}
	request := createRequest(false /*un gziped*/, http.MethodPost, t)
	response := createResponse(false /*un gziped*/, false /* un trunked */, t)
	testHTTPServeSingleRequest(httpImpl, request, response, t)
}

func TestHTTPProxy_FastHTTPServeSinglePostRequest(t *testing.T) {
	httpImpl := httpImpl{eghttp.FastHTTPStr, eghttp.FastHTTPStr}
	request := createRequest(false /*un gziped*/, http.MethodPost, t)
	response := createResponse(false /*un gziped*/, false /* un trunked */, t)
	testHTTPServeSingleRequest(httpImpl, request, response, t)
}

func TestHTTPProxy_MixNetFastHTTPServeSinglePostRequest(t *testing.T) {
	httpImpl := httpImpl{eghttp.NetHTTPStr, eghttp.FastHTTPStr}
	request := createRequest(false /*un gziped*/, http.MethodPost, t)
	response := createResponse(false /*un gziped*/, false /* un trunked */, t)
	testHTTPServeSingleRequest(httpImpl, request, response, t)
}

func TestHTTPProxy_MixFastNetHTTPServeSinglePostRequest(t *testing.T) {
	httpImpl := httpImpl{eghttp.FastHTTPStr, eghttp.NetHTTPStr}
	request := createRequest(false /*un gziped*/, http.MethodPost, t)
	response := createResponse(false /*un gziped*/, false /* un trunked */, t)
	testHTTPServeSingleRequest(httpImpl, request, response, t)
}

//// Test single post gziped request
func TestHTTPPRoxy_NetHTTPServeSingleGzipedRequest(t *testing.T) {
	httpImpl := httpImpl{eghttp.NetHTTPStr, eghttp.NetHTTPStr}
	request := createRequest(true /*gziped*/, http.MethodPost, t)
	response := createResponse(true /*gziped*/, false /* un trunked */, t)
	testHTTPServeSingleRequest(httpImpl, request, response, t)
}

func TestHTTPPRoxy_FastHTTPServeSingleGzipedRequest(t *testing.T) {
	httpImpl := httpImpl{eghttp.NetHTTPStr, eghttp.NetHTTPStr}
	request := createRequest(true /*gziped*/, http.MethodPost, t)
	response := createResponse(true /*gziped*/, false /* un trunked */, t)
	testHTTPServeSingleRequest(httpImpl, request, response, t)
}

func TestHTTPPRoxy_MixFastNetHTTPServeSingleGzipedRequest(t *testing.T) {

	httpImpl := httpImpl{eghttp.FastHTTPStr, eghttp.NetHTTPStr}
	request := createRequest(true /*gziped*/, http.MethodPost, t)
	response := createResponse(true /*gziped*/, false /* un trunked */, t)
	testHTTPServeSingleRequest(httpImpl, request, response, t)
}

func TestHTTPPRoxy_MixNetFastHTTPServeSingleGzipedRequest(t *testing.T) {
	httpImpl := httpImpl{eghttp.NetHTTPStr, eghttp.FastHTTPStr}
	request := createRequest(true /*gziped*/, http.MethodPost, t)
	response := createResponse(true /*gziped*/, false /* un trunked */, t)
	testHTTPServeSingleRequest(httpImpl, request, response, t)
}

//// Test single post trunked response
func TestHTTPPRoxy_NetHTTPServeSingleTrunkedResponse(t *testing.T) {
	httpImpl := httpImpl{eghttp.NetHTTPStr, eghttp.NetHTTPStr}
	request := createRequest(false /*un gziped*/, http.MethodPost, t)
	response := createResponse(false /*un gziped*/, true /* trunked */, t)
	testHTTPServeSingleRequest(httpImpl, request, response, t)
}
func TestHTTPPRoxy_FastHTTPServeSingleTrunkedResponse(t *testing.T) {
	httpImpl := httpImpl{eghttp.FastHTTPStr, eghttp.FastHTTPStr}
	request := createRequest(false /*un gziped*/, http.MethodPost, t)
	response := createResponse(false /*un gziped*/, true /* trunked */, t)
	testHTTPServeSingleRequest(httpImpl, request, response, t)
}

func TestHTTPPRoxy_MixFastNetHTTPServeSingleTrunkedResponse(t *testing.T) {
	httpImpl := httpImpl{eghttp.FastHTTPStr, eghttp.NetHTTPStr}
	request := createRequest(false /*un gziped*/, http.MethodPost, t)
	response := createResponse(false /*un gziped*/, true /* trunked */, t)
	testHTTPServeSingleRequest(httpImpl, request, response, t)
}

func TestHTTPPRoxy_MixNetFastHTTPServeSingleTrunkedResponse(t *testing.T) {
	httpImpl := httpImpl{eghttp.NetHTTPStr, eghttp.FastHTTPStr}
	request := createRequest(false /*un gziped*/, http.MethodPost, t)
	response := createResponse(false /*un gziped*/, true /* trunked */, t)
	testHTTPServeSingleRequest(httpImpl, request, response, t)
}

// EG serves as a proxy, don't uncompress gziped request/response
func testHTTPServeSingleRequest(httpImpl httpImpl, request *request, response *response, t *testing.T) {
	ts := createBackendServer(serverHandler(request, response, t))
	defer ts.Close()

	// httpoutput client will normalize this url
	backendUrl := ts.URL + "{PATH_INFO}//result"
	proxyPath := request.path
	plugins, pipelines := getHTTPProxyCfg(httpImpl, proxyPath, backendUrl, request.method)
	model, scheduler := createPipeline(plugins, pipelines, t)

	defer scheduler.Stop()
	defer model.DismissAllPluginInstances()
	defer scheduler.StopPipeline()
	req := httpRequest(request, t)
	if resp, err := http.DefaultClient.Do(req); err != nil {
		t.Fatalf("post unexpected error: %+v", err)
	} else {
		validateHTTPResp(response, resp, t)
	}
}

type request struct {
	contentType     []string
	body            []byte
	contentEncoding string
	acceptEncoding  []string
	/* userAgent       []string */
	path   string
	method string
}

type response struct {
	contentType     []string
	body            []byte
	contentEncoding string
	trunked         bool
	statusCode      int
}
type httpImpl struct {
	server string
	output string
}

func createRequest(gziped bool, method string, t *testing.T) *request {
	r := &request{
		contentType:    []string{"text/plain; charset=utf-8"},
		acceptEncoding: []string{"gzip", "deflate"}, /* explicitly set acceptEncoding to avoid client uncompress gziped data automatically */
		path:           "/test",
		method:         method,
	}
	if method == http.MethodPost {
		r.body = randBytes()
	}
	if gziped && method == http.MethodPost {
		r.body = gzipBytes(r.body, t)
		r.contentEncoding = "gzip"
		r.path = r.path + "/gzip"
	}
	return r
}

func createResponse(gziped bool, trunked bool, t *testing.T) *response {
	r := &response{
		contentType: []string{"text/plain"},
		body:        randBytes(),
		trunked:     trunked,
		statusCode:  200,
	}
	if gziped {
		r.body = gzipBytes(r.body, t)
		r.contentEncoding = "gzip"
	}
	return r
}

func expectedProxyHeader(request *request) http.Header {
	r := http.Header{
		"Accept-Encoding":  request.acceptEncoding,
		"Content-Type":     request.contentType,
		"User-Agent":       []string{"Go-http-client/1.1"},
		"Content-Encoding": []string{request.contentEncoding},
	}
	if request.body != nil {
		r.Set("Content-Length", strconv.Itoa(len(request.body)))
	}
	return r
}

func expectedResponseHeader(response *response) http.Header {
	r := http.Header{
		"Content-Type": response.contentType,
	}
	if response.contentEncoding != "" {
		r.Set("Content-Encoding", response.contentEncoding)
	}
	if !response.trunked {
		r.Set("Content-Length", strconv.Itoa(len(response.body)))
	} else {
		// The http package automatically decodes chunking when reading not very big(>4k?) response bodies.
	}
	return r
}

func httpRequest(request *request, t *testing.T) *http.Request {
	requestReader := bytes.NewBuffer(request.body)
	proxyUrl := fmt.Sprintf("http://%s:%d%s", defaultHTTPProxyHost, defaultHTTPProxyPort, request.path)
	req, err := http.NewRequest(request.method, proxyUrl, requestReader)
	if err != nil {
		t.Fatalf("http.NewRequest failed: %v", err)
	}
	for _, e := range request.acceptEncoding {
		req.Header.Add("Accept-Encoding", e)
	}
	req.Header.Set("Content-Type", strings.Join(request.contentType, "; "))
	req.Header.Set("Content-Encoding", request.contentEncoding)
	return req
}

// create http.Handler which checks the incoming request and
// respond with `response`
func serverHandler(request *request, response *response, t *testing.T) func(http.ResponseWriter, *http.Request) {

	proxyHeader := expectedProxyHeader(request)
	return func(w http.ResponseWriter, r *http.Request) {
		// check normalized path
		path := request.path + "/result"
		if r.URL.Path != path {
			t.Fatalf("expected path: %v, but got: %v", path, r.URL.Path)
		}

		if body, err := ioutil.ReadAll(r.Body); err != nil {
			t.Fatalf("read body unexpected error: %+v", err)
		} else if string(body) != string(request.body) {
			t.Fatalf("expected body: %+v, but got: %+v", string(request.body), string(body))
		}
		if !reflect.DeepEqual(proxyHeader, r.Header) {
			t.Fatalf("expected proxy header: %+v, but got: %+v", proxyHeader, r.Header)
		}

		if response.trunked {
			// Transfer-Encoding will be handled by the writer implicitly
		} else {
			l := strconv.Itoa(len(response.body))
			w.Header().Set("Content-Length", l)
		}
		if response.contentEncoding != "" {
			w.Header().Set("Content-Encoding", response.contentEncoding)
		}
		for _, typ := range response.contentType {
			w.Header().Add("Content-Type", typ)
		}
		if response.trunked {
			if flusher, ok := w.(http.Flusher); !ok {
				t.Fatalf("expected http.ResponseWriter to be an http.Flusher")
			} else {
				trunk := []byte(response.body)
				len := len(trunk) / 2
				w.Write(trunk[0:len])
				flusher.Flush() // Trigger "chunked" encoding and send a chunk...
				time.Sleep(500 * time.Millisecond)
				w.Write(trunk[len:])
				flusher.Flush() // Trigger "chunked" encoding and send a chunk...
			}
		} else {
			w.Write([]byte(response.body))
		}
	}
}

func validateHTTPResp(response *response, resp *http.Response, t *testing.T) {
	if response.statusCode != resp.StatusCode {
		t.Fatalf("expected status code: %+v, but got: %+v", response.statusCode, resp.StatusCode)
	}
	header := expectedResponseHeader(response)
	if response.trunked {
		resp.Header.Del("Content-Length")
	}
	// don't compare "Date" header
	resp.Header.Del("Date")
	if resp.Header.Get("Server") != "" {
		srv := resp.Header.Get("Server")
		if srv != eghttp.FastHTTPStr { // fasthttp server add "Server" header
			t.Fatalf("expected server header: %+v, but got: %+v", eghttp.FastHTTPStr, srv)
		}
		resp.Header.Del("Server")
	}

	if !reflect.DeepEqual(header, resp.Header) { // it implicitly compares Content-Length
		t.Fatalf("expected response header: %+v, but got: %+v", header, resp.Header)
	}

	if respBody, err := ioutil.ReadAll(resp.Body); err != nil {
		t.Fatalf("read response body unexpected error: %+v", err)
	} else if string(respBody) != string(response.body) {
		t.Fatalf("expected response body: %+v, but got: %+v", string(response.body), string(respBody))
	}
	// The http package automatically decodes chunking when reading response bodies.
	// chunked response don't have Content-Length
	if common.StrInSlice("chunked", resp.TransferEncoding) {
		header.Del("Content-Length")
	}
}
