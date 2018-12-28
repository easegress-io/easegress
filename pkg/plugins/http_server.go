package plugins

import (
	"encoding/json"
	"fmt"
	"math"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/megaease/easegateway/pkg/option"

	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/pipelines"
	"github.com/megaease/easegateway/pkg/task"

	"github.com/erikdubbelboer/fasthttp"
	"golang.org/x/net/netutil"
)

//////// HTTP server interface
type server interface {
	ServeTLS(certFile, keyFile string) error
	Serve() error
	Close() error
	Listener() net.Listener
}

// leverages net/http server, implements server interface
type netHTTPServer struct {
	s *http.Server
	l net.Listener
	m HTTPMux
}

func newNetHTTPServer(c *HTTPServerConfig, l net.Listener, mux HTTPMux) *netHTTPServer {
	ln := netutil.LimitListener(&tcpKeepAliveListener{
		connKeepAlive:    c.ConnKeepAlive,
		connKeepAliveSec: c.ConnKeepAliveSec,
		tcpListener:      l.(*net.TCPListener),
	}, int(c.MaxSimulConns))
	s := http.Server{}
	ns := &netHTTPServer{&s, ln, mux}

	s.Handler = ns
	s.SetKeepAlivesEnabled(c.ConnKeepAlive)
	return ns
}

// implements http.Handler interface
func (n *netHTTPServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := n.createHTTPCtx(w, r)
	n.m.ServeHTTP(ctx)
}

func (n *netHTTPServer) ServeTLS(certFile, keyFile string) error {
	return n.s.ServeTLS(n.l, certFile, keyFile)
}
func (n *netHTTPServer) Serve() error {
	return n.s.Serve(n.l)
}

func (n *netHTTPServer) Close() error {
	return n.s.Close()
}

func (n *netHTTPServer) Listener() net.Listener {
	return n.l
}

func (n *netHTTPServer) createHTTPCtx(w http.ResponseWriter, r *http.Request) HTTPCtx {
	return &NetHTTPCtx{
		Req:    r,
		Writer: w,
	}
}

// leverages fasthttp server, implements server interface
type fastHTTPServer struct {
	s *fasthttp.Server
	l net.Listener
	m HTTPMux
}

func newFastHTTPServer(c *HTTPServerConfig, l net.Listener, mux HTTPMux) *fastHTTPServer {
	s := fasthttp.Server{
		Concurrency:  int(c.MaxSimulConns),
		LogAllErrors: true,
		// set logger to log errors into std logger
		Logger: logger.StdLogger(),
		// TODO(shengdong)
		// open more options here? like Server.MaxRequestBodySize
	}
	if c.ConnKeepAlive {
		// This also limits the maximum duration for idle keep-alive
		// connections.
		s.ReadTimeout = time.Duration(c.ConnKeepAliveSec) * time.Second
	}
	srv := &fastHTTPServer{&s, l, mux}
	s.Handler = srv.ServeFastHTTP

	return srv
}

// implements fasthttp.RequestHandler interface
func (f *fastHTTPServer) ServeFastHTTP(c *fasthttp.RequestCtx) {
	ctx := f.createHTTPCtx(c)
	f.m.ServeHTTP(ctx)
}

func (f *fastHTTPServer) ServeTLS(certFile, keyFile string) error {
	return f.s.ServeTLS(f.l, certFile, keyFile)
}

func (f *fastHTTPServer) Serve() error {
	return f.s.Serve(f.l)
}

func (f *fastHTTPServer) Listener() net.Listener {
	return f.l
}

// Unlike net/http.Server, fasthttp.Server doesn't have `Close()` function,
// which close the listener innerly, so close listener here explicitly
func (f *fastHTTPServer) Close() error {
	if err := f.l.Close(); err != nil {
		return err
	}
	return nil
}

func (f *fastHTTPServer) createHTTPCtx(c *fasthttp.RequestCtx) HTTPCtx {
	return &FastHTTPCtx{
		Ctx:  c,
		Req:  &c.Request,
		Resp: &c.Response,
	}
}

// server factory, creates concrete server interface implementations
func createServer(c *HTTPServerConfig, mux HTTPMux) (server, error) {
	addr := fmt.Sprintf("%s:%d", c.Host, c.Port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("listen at TCP %s failed: ", err)
	}

	if c.typ == FastHTTP {
		return newFastHTTPServer(c, ln, mux), nil
	} else {
		return newNetHTTPServer(c, ln, mux), nil
	}
}

////////
type HTTPServerConfig struct {
	PluginCommonConfig
	Host             string      `json:"host"`
	Port             uint16      `json:"port"` // up to 65535
	MuxType          muxType     `json:"mux_type"`
	MuxConfig        interface{} `json:"mux_config"`
	CertFile         string      `json:"cert_file"`
	KeyFile          string      `json:"key_file"`
	ConnKeepAlive    bool        `json:"keepalive"`
	ConnKeepAliveSec uint16      `json:"keepalive_sec"` // up to 65535
	Type             string      `json:"type"`
	typ              HTTPType
	// TODO: Adds keepalive_requests support
	MaxSimulConns uint32 `json:"max_connections"` // up to 2147483647 really

	certFilePath, keyFilePath string
	https                     bool
	muxConf                   interface{}
}

func HTTPServerConfigConstructor() Config {
	return &HTTPServerConfig{
		Host:    "localhost",
		Port:    10080,
		MuxType: regexpMuxType,
		Type:    NetHTTPStr,
		MuxConfig: reMuxConfig{
			CacheKeyComplete: false,
			CacheMaxCount:    1024,
		},
		ConnKeepAlive:    true,
		ConnKeepAliveSec: 10,
		MaxSimulConns:    10240,
	}
}

func (c *HTTPServerConfig) Prepare(pipelineNames []string) error {
	err := c.PluginCommonConfig.Prepare(pipelineNames)
	if err != nil {
		return err
	}

	ts := strings.TrimSpace
	c.Host = ts(c.Host)
	c.CertFile = ts(c.CertFile)
	c.KeyFile = ts(c.KeyFile)
	c.Type = ts(c.Type)

	if len(c.Host) == 0 {
		return fmt.Errorf("invalid host")
	}

	if len(c.CertFile) != 0 || len(c.KeyFile) != 0 {
		c.certFilePath = filepath.Join(option.CertDir, c.CertFile)
		c.keyFilePath = filepath.Join(option.CertDir, c.KeyFile)

		if s, err := os.Stat(c.certFilePath); os.IsNotExist(err) || s.IsDir() {
			return fmt.Errorf("cert file %s not found", c.CertFile)
		}

		if s, err := os.Stat(c.keyFilePath); os.IsNotExist(err) || s.IsDir() {
			return fmt.Errorf("key file %s not found", c.KeyFile)
		}

		c.https = true
	}

	if c.Port == 0 {
		return fmt.Errorf("invalid port")
	}

	typ, err := ParseHTTPType(c.Type)
	if err != nil {
		return fmt.Errorf("parse http implementation failed: %v", err)
	}
	c.typ = typ

	var muxConf interface{}
	switch c.MuxType {
	case regexpMuxType:
		muxConf = new(reMuxConfig)
	case paramMuxType:
		muxConf = new(paramMuxConfig)
	default:
		return fmt.Errorf("unsupported mux type")
	}

	muxBuff, err := json.Marshal(c.MuxConfig)
	if err != nil {
		return err
	}
	err = json.Unmarshal(muxBuff, muxConf)
	if err != nil {
		return err
	}
	c.muxConf = muxConf

	if c.ConnKeepAliveSec == 0 {
		return fmt.Errorf("invalid connection keep-alive period")
	}

	if c.MaxSimulConns == 0 ||
		c.MaxSimulConns > math.MaxInt32 { // defense overflow on make(chan, c.MaxSimulConns int)
		return fmt.Errorf("invalid max simultaneous connection amount")
	}

	return nil
}

type httpServer struct {
	conf   *HTTPServerConfig
	addr   string
	server server
	mux    HTTPMux
	closed bool
}

func httpServerConstructor(conf Config) (Plugin, PluginType, bool, error) {
	c, ok := conf.(*HTTPServerConfig)
	if !ok {
		return nil, ProcessPlugin, true, fmt.Errorf(
			"config type want *HTTPServerConfig got %T", conf)
	}

	h := &httpServer{
		conf: c,
	}

	h.addr = fmt.Sprintf("%s:%d", c.Host, c.Port)
	var err error
	switch h.conf.MuxType {
	case regexpMuxType:
		muxConf, ok := h.conf.muxConf.(*reMuxConfig)
		if !ok {
			logger.Errorf("[BUG: want *reMuxConfig got %T]", h.conf.muxConf)
			return nil, ProcessPlugin, true, fmt.Errorf(
				"construct regexp mux failed: mux config type want *reMuxConfig got %T", h.conf.muxConf)
		}
		h.mux, err = newREMux(muxConf)
		if err != nil {
			return nil, ProcessPlugin, true, fmt.Errorf("construct regexp mux failed: %v", err)
		}
	case paramMuxType:
		muxConf, ok := h.conf.muxConf.(*paramMuxConfig)
		if !ok {
			logger.Errorf("[BUG: want *paramMuxConfig got %T]", h.conf.muxConf)
			return nil, ProcessPlugin, true, fmt.Errorf(
				"construct param mux failed: mux config type want *paramMuxConfig got %T", h.conf.muxConf)
		}
		h.mux, err = newParamMux(muxConf)
		if err != nil {
			return nil, ProcessPlugin, true, fmt.Errorf("construct param mux failed: %v", err)
		}
	default:
		return nil, ProcessPlugin, true, fmt.Errorf("unsupported mux type") //defensive
	}
	h.server, err = createServer(c, h.mux)
	if err != nil {
		return nil, ProcessPlugin, true, fmt.Errorf("create http server failed: %s", err)
	}

	done := make(chan error)
	defer close(done)

	server_startup_notifier := func(e error) {
		defer func() {
			// server will be shutdown during close, ignore safely
			recover()
		}()
		done <- e
	}

	if c.https {
		logger.Infof("[https server %s is listening at %s]", c.Name, h.addr)

		go func() {
			err := h.server.ServeTLS(c.certFilePath, c.keyFilePath)
			if !h.closed && err != nil {
				logger.Errorf("[https server listens %s failed: %v]", h.addr, err)
			}
			server_startup_notifier(err)
		}()
	} else {
		logger.Infof("[http server %s is listening at %s]", c.Name, h.addr)

		go func() {
			err := h.server.Serve()
			if !h.closed && err != nil {
				logger.Errorf("[http server listens %s failed: %v]", h.addr, err)
			}
			server_startup_notifier(err)
		}()
	}

	select {
	case err = <-done:
	case <-time.After(500 * time.Millisecond): // wait fast failure of server startup, hate this
	}

	if err != nil {
		h.server.Listener().Close()
		h.closed = true
		return nil, ProcessPlugin, true, err
	}

	return h, ProcessPlugin, true, nil
}

func (h *httpServer) Prepare(ctx pipelines.PipelineContext) {
	pipeline_rtable := getPipelineRouteTable(ctx, h.Name())
	if pipeline_rtable != nil {
		h.mux.AddFuncs(ctx, pipeline_rtable)
	}

	storeHTTPServerMux(ctx, h.Name(), h.mux)
	storeHTTPServerGoneNotifier(ctx, h.Name(), make(chan struct{}))
}

func (h *httpServer) Run(ctx pipelines.PipelineContext, t task.Task) error {
	/* FIXME(zhiyan): don't check for exceptional case, to help performance for normal case
	if len(ctx.PluginNames()) == 1 { // only myself, wrong usage
		// yield to help stupid operator correct this by pipeline update
		time.Sleep(time.Millisecond)
	} */

	return nil
}

func (h *httpServer) Name() string {
	return h.conf.PluginName()
}

func (h *httpServer) CleanUp(ctx pipelines.PipelineContext) {
	mux := getHTTPServerMux(ctx, h.Name(), true)
	if mux == nil {
		// doesn't make sense, defensive
		return
	}

	pipeline_rtable := mux.DeleteFuncs(ctx)
	if pipeline_rtable != nil {
		storePipelineRouteTable(ctx, h.Name(), pipeline_rtable)
	}

	notifier := getHTTPServerGoneNotifier(ctx, h.Name(), true)
	if notifier != nil {
		close(notifier)
	}
}

func (h *httpServer) Close() {
	h.closed = true

	err := h.server.Close()
	if err != nil {
		logger.Errorf("[shut server listens at %s down failed: %v]", h.addr, err)
	} else {
		logger.Infof("[server listens at %s is shut down]", h.addr)
	}

	// wait close really by spin
	for {
		ln, err := net.Listen("tcp", h.addr)
		if err != nil {
			continue
		}

		ln.Close()
		break
	}
}

////

type tcpKeepAliveListener struct {
	connKeepAlive    bool
	connKeepAliveSec uint16
	tcpListener      *net.TCPListener
}

func (ln tcpKeepAliveListener) Accept() (c net.Conn, err error) {
	tc, err := ln.tcpListener.AcceptTCP()
	if err != nil {
		return
	}

	tc.SetKeepAlive(ln.connKeepAlive)
	if ln.connKeepAlive {
		tc.SetKeepAlivePeriod(time.Duration(ln.connKeepAliveSec) * time.Second)
		// The connection is TCP_NODELAY by default, meaning that data is
		// sent as soon as possible after a Write.
	}

	return tc, nil
}

func (ln tcpKeepAliveListener) Close() error {
	return ln.tcpListener.Close()
}

func (ln tcpKeepAliveListener) Addr() net.Addr {
	return ln.tcpListener.Addr()
}

////

func storeHTTPServerMux(ctx pipelines.PipelineContext, pluginName string, mux HTTPMux) error {
	bucket := ctx.DataBucket(pluginName, pipelines.DATA_BUCKET_FOR_ALL_PLUGIN_INSTANCE)
	_, err := bucket.BindData(HTTP_SERVER_MUX_BUCKET_KEY, mux)
	if err != nil {
		logger.Warnf("[BUG: store the mux of http server %s for pipeline %s failed, "+
			"ignored to provide mux: %v]", pluginName, ctx.PipelineName(), err)
		return err
	}

	return nil
}

func getHTTPServerMux(ctx pipelines.PipelineContext, pluginName string, required bool) HTTPMux {
	bucket := ctx.DataBucket(pluginName, pipelines.DATA_BUCKET_FOR_ALL_PLUGIN_INSTANCE)
	mux := bucket.QueryData(HTTP_SERVER_MUX_BUCKET_KEY)

	ret, ok := mux.(HTTPMux)
	if !ok && required {
		logger.Errorf("[the mux of http server %s for pipeline %s is invalid]",
			pluginName, ctx.PipelineName())
		return nil
	}

	return ret
}

func storePipelineRouteTable(ctx pipelines.PipelineContext, pluginName string,
	pipelineRTable []*HTTPMuxEntry) error {

	bucket := ctx.DataBucket(pluginName, pipelines.DATA_BUCKET_FOR_ALL_PLUGIN_INSTANCE)
	_, err := bucket.BindData(HTTP_SERVER_PIPELINE_ROUTE_TABLE_BUCKET_KEY, pipelineRTable)
	if err != nil {
		logger.Errorf("[BUG: store the route table of pipeline %s for http server %s failed: %v]",
			ctx.PipelineName(), pluginName, err)
		return err
	}

	return nil
}

func getPipelineRouteTable(ctx pipelines.PipelineContext,
	pluginName string) []*HTTPMuxEntry {

	bucket := ctx.DataBucket(pluginName, pipelines.DATA_BUCKET_FOR_ALL_PLUGIN_INSTANCE)
	pipelineRTable := bucket.QueryData(HTTP_SERVER_PIPELINE_ROUTE_TABLE_BUCKET_KEY)

	if pipelineRTable == nil {
		return nil
	}

	ret, ok := pipelineRTable.([]*HTTPMuxEntry)
	if !ok {
		logger.Errorf("[the route table of pipeline %s for http server %s is invalid]",
			ctx.PipelineName(), pluginName)
		return nil
	}

	return ret
}

func storeHTTPServerGoneNotifier(ctx pipelines.PipelineContext, pluginName string, notifier chan struct{}) error {
	bucket := ctx.DataBucket(pluginName, pipelines.DATA_BUCKET_FOR_ALL_PLUGIN_INSTANCE)
	_, err := bucket.BindData(HTTP_SERVER_GONE_NOTIFIER_BUCKET_KEY, notifier)
	if err != nil {
		logger.Warnf("[BUG: store the close notifier of http server %s for pipeline %s failed, "+
			"ignored to provide close notifier: %v]", pluginName, ctx.PipelineName(), err)
		return err
	}

	return nil
}

func getHTTPServerGoneNotifier(ctx pipelines.PipelineContext, pluginName string, required bool) chan struct{} {
	bucket := ctx.DataBucket(pluginName, pipelines.DATA_BUCKET_FOR_ALL_PLUGIN_INSTANCE)
	notifier := bucket.QueryData(HTTP_SERVER_GONE_NOTIFIER_BUCKET_KEY)

	ret, ok := notifier.(chan struct{})
	if !ok && required {
		logger.Errorf("[the close notifier of http server %s for pipeline %s is invalid]",
			pluginName, ctx.PipelineName())
		return nil
	}

	return ret
}
