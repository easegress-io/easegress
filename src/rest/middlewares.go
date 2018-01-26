package rest

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"time"

	"github.com/ant0ine/go-json-rest/rest"
	"github.com/emicklei/go-restful"

	"cluster/gateway"
	"logger"
	"version"
)

var restStack = []rest.Middleware{
	&accessLogMiddleware{},
	&rest.TimerMiddleware{},
	&rest.RecorderMiddleware{},
	&rest.PoweredByMiddleware{
		XPoweredBy: fmt.Sprintf("EaseGateway/rest-api/%s-%s", version.RELEASE, version.COMMIT),
	},
	&rest.RecoverMiddleware{},
	&rest.GzipMiddleware{},
	&rest.ContentTypeCheckerMiddleware{},
}

////

type clusterAvailabilityMiddleware struct {
	gc *gateway.GatewayCluster
}

// implement go-restful RouteFunction
func (cam *clusterAvailabilityMiddleware) Process(req *restful.Request, resp *restful.Response, chain *restful.FilterChain) {
	if cam.gc == nil || cam.gc.Stopped() {
		resp.WriteHeaderAndJson(http.StatusServiceUnavailable, errorResponse{"service unavailable"}, restful.MIME_JSON)
		return
	}
	chain.ProcessFilter(req, resp)
}

// implement go-json-rest/rest Middleware interface
func (cam *clusterAvailabilityMiddleware) MiddlewareFunc(h rest.HandlerFunc) rest.HandlerFunc {
	return func(w rest.ResponseWriter, r *rest.Request) {
		if cam.gc == nil || cam.gc.Stopped() {
			rest.Error(w, "service unavailable", http.StatusServiceUnavailable)
			return
		}

		// call the handler
		h(w, r)
	}
}

type standaloneAvailabilityMiddleware struct {
	gc *gateway.GatewayCluster
}

// implement go-restful RouteFunction
func (sam *standaloneAvailabilityMiddleware) Process(req *restful.Request, resp *restful.Response, chain *restful.FilterChain) {
	if sam.gc != nil && !sam.gc.Stopped() && req.Request.Method != "GET" {
		// standalone admin api only supports retrieve operations when running in cluster mode
		resp.WriteHeaderAndJson(http.StatusServiceUnavailable, errorResponse{"service unavailable"}, restful.MIME_JSON)
		return
	}
	chain.ProcessFilter(req, resp)
}

func (sam *standaloneAvailabilityMiddleware) MiddlewareFunc(h rest.HandlerFunc) rest.HandlerFunc {
	return func(w rest.ResponseWriter, r *rest.Request) {
		if sam.gc != nil && !sam.gc.Stopped() && r.Method != "GET" {
			// standalone admin api only supports retrieve operations when running in cluster mode
			rest.Error(w, "service unavailable", http.StatusServiceUnavailable)
			return
		}

		// call the handler
		h(w, r)
	}
}

////

type accessLogMiddleware struct{}

// implement go-restful RouteFunction
func (cam *accessLogMiddleware) Process(req *restful.Request, resp *restful.Response, chain *restful.FilterChain) {
	dump, err := httputil.DumpRequest(req.Request, true)
	if err != nil {
		logger.Warnf("[dump rest api request for log failed: %v]", err)
	}

	chain.ProcessFilter(req, resp)
	logger.RESTAccess(req.Request, resp.StatusCode(), int64(resp.ContentLength()),
		req.Attribute("START_TIME").(*time.Time), req.Attribute("ELAPSED_TIME").(*time.Duration), dump)
}

// implement go-json-rest/rest Middleware interface
func (cam *accessLogMiddleware) MiddlewareFunc(h rest.HandlerFunc) rest.HandlerFunc {
	return func(w rest.ResponseWriter, r *rest.Request) {
		dump, err := httputil.DumpRequest(r.Request, true)
		if err != nil {
			logger.Warnf("[dump rest api request for log failed: %v]", err)
		}

		// call the handler
		h(w, r)

		logger.RESTAccess(r.Request, r.Env["STATUS_CODE"].(int), r.Env["BYTES_WRITTEN"].(int64),
			r.Env["START_TIME"].(*time.Time), r.Env["ELAPSED_TIME"].(*time.Duration), dump)
	}
}

////
type timerFilter struct{}

func (t *timerFilter) Process(req *restful.Request, resp *restful.Response, chain *restful.FilterChain) {
	start := time.Now()
	req.SetAttribute("START_TIME", &start)
	chain.ProcessFilter(req, resp)
	elapsed := time.Since(start)
	req.SetAttribute("ELAPSED_TIME", &elapsed)
}

////
type poweredByFilter struct {
	XPoweredBy string
}

func (p *poweredByFilter) Process(req *restful.Request, resp *restful.Response, chain *restful.FilterChain) {
	if p.XPoweredBy != "" {
		resp.Header().Add("X-Powered-By", p.XPoweredBy)
	}
	chain.ProcessFilter(req, resp)
}
