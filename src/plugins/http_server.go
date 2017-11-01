package plugins

import (
	"common"
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
	Host             string `json:"host"`
	Port             uint16 `json:"port"` // up to 65535
	CertFile         string `json:"cert_file"`
	KeyFile          string `json:"key_file"`
	ConnKeepAlive    bool   `json:"keepalive"`
	ConnKeepAliveSec uint16 `json:"keepalive_sec"` // up to 65535
	// TODO: Adds keepalive_requests support
	MaxSimulConns uint32 `json:"max_connections"` // up to 4294967295

	certFilePath, keyFilePath string
	https                     bool
}

func httpServerConfigConstructor() plugins.Config {
	return &httpServerConfig{
		Host:             "localhost",
		Port:             10080,
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

	h.mux = newMux()

	h.server = &http.Server{
		Handler: h.mux,
	}

	h.server.SetKeepAlivesEnabled(c.ConnKeepAlive)
	if c.ConnKeepAlive {
		h.server.IdleTimeout = time.Duration(c.ConnKeepAliveSec) * time.Second
	}

	logger.Debugf("[the server is starting at %s]", h.addr)

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
		go func() {
			err := h.server.ServeTLS(ln, c.certFilePath, c.keyFilePath)
			if !h.closed && err != nil {
				logger.Errorf("[https server listens %s failed: %v]", h.addr, err)
			}
			server_startup_notifier(err)
		}()
	} else {
		go func() {
			err := h.server.Serve(ln)
			if !h.closed && err != nil {
				logger.Errorf("[http server listens %s failed: %v]", h.addr, err)
			}
			server_startup_notifier(err)
		}()
	}

	// This is a trick way, but it's fine, in most case error will
	// happen at Listen() or doing preparation in Serve() at start
	select {
	case err = <-done:
	case <-time.After(2 * time.Second): // FIXME: I hate this kind of magic number, but Serve() blocks
	}

	if err != nil {
		h.listener.Close()
		h.closed = true
		return nil, err
	}

	return h, nil
}

func (h *httpServer) Prepare(ctx pipelines.PipelineContext) {
	mux, err := storeHTTPServerMux(ctx, h.Name(), h.mux)
	if err != nil {
		h.server.Handler = newMux() // empty route table
		return
	}

	// reuse existing mux, if exists, which is created and used before the server plugin update
	h.server.Handler = mux
}

func (h *httpServer) Run(ctx pipelines.PipelineContext, t task.Task) (task.Task, error) {
	// Nothing to do
	return t, nil
}

func (h *httpServer) Name() string {
	return h.conf.PluginName()
}

func (h *httpServer) CleanUp(ctx pipelines.PipelineContext) {
	// Nothing to do.
}

func (h *httpServer) Close(contexts map[string]pipelines.PipelineContext) {
	h.closed = true

	err := h.server.Close()
	if err != nil {
		logger.Errorf("[shut server listens at %s down failed: %v]", h.addr, err)
	} else {
		logger.Debugf("[server listens at %s is shut down]", h.addr)
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

func storeHTTPServerMux(ctx pipelines.PipelineContext, pluginName string,
	defaultMux plugins.HTTPMux) (plugins.HTTPMux, error) {

	bucket := ctx.DataBucket(pluginName, pipelines.DATA_BUCKET_FOR_ALL_PLUGIN_INSTANCE)
	mux, err := bucket.QueryDataWithBindDefault(plugins.HTTP_SERVER_MUX_BUCKET_KEY,
		func() interface{} {
			return defaultMux
		})
	if err != nil {
		logger.Warnf("[BUG: query the mux of http server %s for pipeline %s failed, "+
			"ignored to provide mux: %v]", pluginName, ctx.PipelineName(), err)
		return nil, err
	}

	return mux.(plugins.HTTPMux), nil
}
