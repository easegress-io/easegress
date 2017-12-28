package rest

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"time"

	"github.com/ant0ine/go-json-rest/rest"

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
