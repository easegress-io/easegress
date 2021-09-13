package tcpproxy

import (
	stdctxt "context"
	"fmt"
	"net"
	"net/http"
	"net/url"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/httppipeline"
)

const (
	// Kind is the kind of HTTPPipeline.
	Kind = "TCPProxy"
)

type (
	Server struct {
		URL string `yaml:"url"`
	}

	Spec struct {
		Servers []Server
	}

	TCPProxy struct {
		filterSpec *httppipeline.FilterSpec // The filter spec in pipeline level, which has two more fiels: kind and name.
		spec       *Spec                    // The filter spec in its own level.
	}
)

// init registers itself to pipeline registry.
func init() {
	httppipeline.Register(&TCPProxy{})
}

// Kind returns the kind of HeaderCounter.
func (tp *TCPProxy) Kind() string {
	return Kind
}

// DefaultSpec returns default spec of HeaderCounter.
func (tp *TCPProxy) DefaultSpec() interface{} {
	return &Spec{}
}

// Description returns the description of HeaderCounter.
func (tp *TCPProxy) Description() string {
	return "TCPProxy sets a group of backend servers connected by TCP."
}

// Results returns the results of HeaderCounter.
func (tp *TCPProxy) Results() []string { return nil }

// Init initializes HeaderCounter.
func (tp *TCPProxy) Init(filterSpec *httppipeline.FilterSpec) {
	logger.Infof("Init with %v\n", filterSpec)
	tp.filterSpec, tp.spec = filterSpec, filterSpec.FilterSpec().(*Spec)
	tp.reload()
}

// Inherit inherits previous generation of HeaderCounter.
func (tp *TCPProxy) Inherit(filterSpec *httppipeline.FilterSpec, prevGeneration httppipeline.Filter) {
	prevGeneration.Close()
	tp.Init(filterSpec)
}

func (tp *TCPProxy) Handle(ctx context.HTTPContext) (result string) {
	ss := tp.spec.Servers
	urlObj, err := url.Parse(ss[0].URL)
	if err != nil {
		// TODO move to Validate()
		panic(err)
	}

	logger.Infof("handled by %s\n", ss[0].URL)

	defaultDialer := new(net.Dialer)
	var dial = func(ctx stdctxt.Context, _, _ string) (net.Conn, error) {
		return defaultDialer.DialContext(ctx, urlObj.Scheme, urlObj.Host)
	}
	// 1. declare a transport
	tr := &http.Transport{
		DialContext: dial,
	}
	client := &http.Client{Transport: tr}

	origReq := ctx.Request().Std()
	u := fmt.Sprintf("http://%s%s", origReq.Host, origReq.URL)

	req, err := http.NewRequest(origReq.Method, u, origReq.Body)
	if err != nil {
		panic(err)
	}
	req.Header = origReq.Header.Clone()
	req.Host = origReq.Host

	// 2. use it send request to tcp server
	resp, err := client.Do(req)
	if err != nil {
		// TODO gracefully handle the error
		panic(err)
	}

	// 3. write the response back to the HTTPContext
	ctx.Lock()
	defer ctx.Unlock()
	ctx.Response().SetStatusCode(resp.StatusCode)
	ctx.Response().Header().AddFromStd(resp.Header)
	ctx.Response().SetBody(resp.Body)

	return ctx.CallNextHandler("")
}

func (tp *TCPProxy) reload() {
	logger.Infof("reload tcp proxy")
}

// Status returns status.
func (tp *TCPProxy) Status() interface{} {
	return nil
}

// Close closes HeaderCounter.
func (tp *TCPProxy) Close() {}
