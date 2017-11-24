package plugins

import (
	"common"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hexdecteam/easegateway-types/pipelines"
	"github.com/hexdecteam/easegateway-types/plugins"
	"github.com/hexdecteam/easegateway-types/task"
	"golang.org/x/net/netutil"

	"logger"
)

type httpServerConfig struct {
	common.PluginCommonConfig
	Host             string      `json:"host"`
	Port             uint16      `json:"port"` // up to 65535
	MuxType          muxType     `json:"mux_type"`
	MuxConfig        interface{} `json:"mux_config"`
	CertFile         string      `json:"cert_file"`
	KeyFile          string      `json:"key_file"`
	ConnKeepAlive    bool        `json:"keepalive"`
	ConnKeepAliveSec uint16      `json:"keepalive_sec"` // up to 65535
	// TODO: Adds keepalive_requests support
	MaxSimulConns uint32 `json:"max_connections"` // up to 4294967295

	certFilePath, keyFilePath string
	https                     bool
	muxConf                   interface{}
}

func httpServerConfigConstructor() plugins.Config {
	return &httpServerConfig{
		Host:    "localhost",
		Port:    10080,
		MuxType: regexpMuxType,
		MuxConfig: reMuxConfig{
			CacheKeyComplete: false,
			CacheMaxCount:    1024,
		},
		ConnKeepAlive:    true,
		ConnKeepAliveSec: 10,
		MaxSimulConns:    1024,
	}
}

func (c *httpServerConfig) Prepare(pipelineNames []string) error {
	err := c.PluginCommonConfig.Prepare(pipelineNames)
	if err != nil {
		return err
	}

	ts := strings.TrimSpace
	c.Host = ts(c.Host)
	c.CertFile = ts(c.CertFile)
	c.KeyFile = ts(c.KeyFile)

	if len(c.Host) == 0 {
		return fmt.Errorf("invalid host")
	}

	if len(c.CertFile) != 0 || len(c.KeyFile) != 0 {
		c.certFilePath = filepath.Join(common.CERT_HOME_DIR, c.CertFile)
		c.keyFilePath = filepath.Join(common.CERT_HOME_DIR, c.KeyFile)

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

	if c.MaxSimulConns == 0 {
		return fmt.Errorf("invalid max simultaneous connection amount")
	}

	return nil
}

type httpServer struct {
	conf     *httpServerConfig
	addr     string
	listener net.Listener
	server   *http.Server
	mux      plugins.HTTPMux
	closed   bool
}

func httpServerConstructor(conf plugins.Config) (plugins.Plugin, error) {
	c, ok := conf.(*httpServerConfig)
	if !ok {
		return nil, fmt.Errorf("config type want *httpServerConfig got %T", conf)
	}

	h := &httpServer{
		conf: c,
	}

	h.addr = fmt.Sprintf("%s:%d", c.Host, c.Port)

	ln, err := net.Listen("tcp", h.addr)
	if err != nil {
		return nil, err
	}

	h.listener = netutil.LimitListener(&tcpKeepAliveListener{
		connKeepAlive:    c.ConnKeepAlive,
		connKeepAliveSec: c.ConnKeepAliveSec,
		tcpListener:      ln.(*net.TCPListener),
	}, int(c.MaxSimulConns))

	switch h.conf.MuxType {
	case regexpMuxType:
		muxConf, ok := h.conf.muxConf.(*reMuxConfig)
		if !ok {
			logger.Errorf("[BUG: want *reMuxConfig got %T]", h.conf.muxConf)
			return nil, fmt.Errorf("construct regexp mux failed: mux config type want *reMuxConfig got %T",
				h.conf.muxConf)
		}
		h.mux, err = newREMux(muxConf)
		if err != nil {
			return nil, fmt.Errorf("construct regexp mux failed: %v", err)
		}
	case paramMuxType:
		muxConf, ok := h.conf.muxConf.(*paramMuxConfig)
		if !ok {
			logger.Errorf("[BUG: want *paramMuxConfig got %T]", h.conf.muxConf)
			return nil, fmt.Errorf("construct param mux failed: mux config type want *paramMuxConfig got %T",
				h.conf.muxConf)
		}
		h.mux, err = newParamMux(muxConf)
		if err != nil {
			return nil, fmt.Errorf("construct param mux failed: %v", err)
		}
	default:
		return nil, fmt.Errorf("unsupported mux type") //defensive
	}

	h.server = &http.Server{
		Handler: h.mux,
	}

	h.server.SetKeepAlivesEnabled(c.ConnKeepAlive)
	if c.ConnKeepAlive {
		h.server.IdleTimeout = time.Duration(c.ConnKeepAliveSec) * time.Second
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
			err := h.server.ServeTLS(h.listener, c.certFilePath, c.keyFilePath)
			if !h.closed && err != nil {
				logger.Errorf("[https server listens %s failed: %v]", h.addr, err)
			}
			server_startup_notifier(err)
		}()
	} else {
		logger.Infof("[http server %s is listening at %s]", c.Name, h.addr)

		go func() {
			err := h.server.Serve(h.listener)
			if !h.closed && err != nil {
				logger.Errorf("[http server listens %s failed: %v]", h.addr, err)
			}
			server_startup_notifier(err)
		}()
	}

	select {
	case err = <-done:
	default:
	}

	if err != nil {
		h.listener.Close()
		h.closed = true
		return nil, err
	}

	return h, nil
}

func (h *httpServer) Prepare(ctx pipelines.PipelineContext) {
	pipeline_rtable := getPipelineRouteTable(ctx, h.Name())
	if pipeline_rtable != nil {
		h.mux.AddFuncs(ctx.PipelineName(), pipeline_rtable)
	}

	storeHTTPServerMux(ctx, h.Name(), h.mux)
	storeHTTPServerGoneNotifier(ctx, h.Name(), make(chan struct{}))
}

func (h *httpServer) Run(ctx pipelines.PipelineContext, t task.Task) error {
	// Nothing to do
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

	pipeline_rtable := mux.DeleteFuncs(ctx.PipelineName())
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

func storeHTTPServerMux(ctx pipelines.PipelineContext, pluginName string, mux plugins.HTTPMux) error {
	bucket := ctx.DataBucket(pluginName, pipelines.DATA_BUCKET_FOR_ALL_PLUGIN_INSTANCE)
	_, err := bucket.BindData(plugins.HTTP_SERVER_MUX_BUCKET_KEY, mux)
	if err != nil {
		logger.Warnf("[BUG: store the mux of http server %s for pipeline %s failed, "+
			"ignored to provide mux: %v]", pluginName, ctx.PipelineName(), err)
		return err
	}

	return nil
}

func getHTTPServerMux(ctx pipelines.PipelineContext, pluginName string, required bool) plugins.HTTPMux {
	bucket := ctx.DataBucket(pluginName, pipelines.DATA_BUCKET_FOR_ALL_PLUGIN_INSTANCE)
	mux := bucket.QueryData(plugins.HTTP_SERVER_MUX_BUCKET_KEY)

	ret, ok := mux.(plugins.HTTPMux)
	if !ok && required {
		logger.Errorf("[the mux of http server %s for pipeline %s is invalid]",
			pluginName, ctx.PipelineName())
		return nil
	}

	return ret
}

func storePipelineRouteTable(ctx pipelines.PipelineContext, pluginName string,
	pipelineRTable []*plugins.HTTPMuxEntry) error {

	bucket := ctx.DataBucket(pluginName, pipelines.DATA_BUCKET_FOR_ALL_PLUGIN_INSTANCE)
	_, err := bucket.BindData(plugins.HTTP_SERVER_PIPELINE_ROUTE_TABLE_BUCKET_KEY, pipelineRTable)
	if err != nil {
		logger.Errorf("[BUG: store the route table of pipeline %s for http server %s failed: %v]",
			ctx.PipelineName(), pluginName, err)
		return err
	}

	return nil
}

func getPipelineRouteTable(ctx pipelines.PipelineContext,
	pluginName string) []*plugins.HTTPMuxEntry {

	bucket := ctx.DataBucket(pluginName, pipelines.DATA_BUCKET_FOR_ALL_PLUGIN_INSTANCE)
	pipelineRTable := bucket.QueryData(plugins.HTTP_SERVER_PIPELINE_ROUTE_TABLE_BUCKET_KEY)

	if pipelineRTable == nil {
		return nil
	}

	ret, ok := pipelineRTable.([]*plugins.HTTPMuxEntry)
	if !ok {
		logger.Errorf("[the route table of pipeline %s for http server %s is invalid]",
			ctx.PipelineName(), pluginName)
		return nil
	}

	return ret
}

func storeHTTPServerGoneNotifier(ctx pipelines.PipelineContext, pluginName string, notifier chan struct{}) error {
	bucket := ctx.DataBucket(pluginName, pipelines.DATA_BUCKET_FOR_ALL_PLUGIN_INSTANCE)
	_, err := bucket.BindData(plugins.HTTP_SERVER_GONE_NOTIFIER_BUCKET_KEY, notifier)
	if err != nil {
		logger.Warnf("[BUG: store the close notifier of http server %s for pipeline %s failed, "+
			"ignored to provide close notifier: %v]", pluginName, ctx.PipelineName(), err)
		return err
	}

	return nil
}

func getHTTPServerGoneNotifier(ctx pipelines.PipelineContext, pluginName string, required bool) chan struct{} {
	bucket := ctx.DataBucket(pluginName, pipelines.DATA_BUCKET_FOR_ALL_PLUGIN_INSTANCE)
	notifier := bucket.QueryData(plugins.HTTP_SERVER_GONE_NOTIFIER_BUCKET_KEY)

	ret, ok := notifier.(chan struct{})
	if !ok && required {
		logger.Errorf("[the close notifier of http server %s for pipeline %s is invalid]",
			pluginName, ctx.PipelineName())
		return nil
	}

	return ret
}
