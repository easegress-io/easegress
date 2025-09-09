/*
 * Copyright (c) 2017, The Easegress Authors
 * All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

// Package routers provides the router interface and the implementation of
// different routing policies.
package routers

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/megaease/easegress/v2/pkg/protocols"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot"
)

type (
	// Kind contains the meta data and functions of a router kind.
	Kind struct {
		// Name is the name of the router kind.
		Name string

		// Description is the description of the router.
		Description string

		// CreateInstance creates a new router instance of the kind.
		CreateInstance func(rules Rules) Router
	}

	// Router is the interface for route search.
	Router interface {
		// Search performs a route search and populates the context with search-related information.
		Search(context *RouteContext)
	}

	// Route is the corresponding route interface for different routing policies.
	Route interface {
		protocols.Route

		// Rewrite for path rewriting.
		Rewrite(context *RouteContext)
		// GetBackend is used to get the backend corresponding to the route.
		GetBackend() string
		// GetClientMaxBodySize is used to get the clientMaxBodySize corresponding to the route.
		GetClientMaxBodySize() int64

		// NOTE: Currently we only support path information in readonly.
		// Without further requirements, we choose not to expose too much information.

		// GetExactPath is used to get the exact path corresponding to the route.
		GetExactPath() string
		// GetPathPrefix is used to get the path prefix corresponding to the route.
		GetPathPrefix() string
		// GetPathRegexp is used to get the path regexp corresponding to the route.
		GetPathRegexp() string
	}

	// Params are used to store the variables in the search path and their corresponding values.
	Params struct {
		Keys, Values []string
	}

	// RouteContext is the context container in the route search
	RouteContext struct {
		// Request is httpprot.Request.
		Request *httpprot.Request
		// Path represents the path of the request.
		Path string
		// Method represents the MethodType corresponding to the http method.
		Method  MethodType
		host    string
		queries url.Values

		// Params are used to store the variables in the search path and their corresponding values.
		Params   Params
		captures map[string]string

		// Cacheable means whether the route can be cached or not.
		Cacheable bool
		// Route represents the results of this search
		Route                                                     Route
		HeaderMismatch, MethodMismatch, QueryMismatch, IPMismatch bool
	}

	// MethodType represents the bit-operated representation of the http method.
	MethodType uint
)

const (
	mSTUB MethodType = 1 << iota
	mCONNECT
	mDELETE
	mGET
	mHEAD
	mOPTIONS
	mPATCH
	mPOST
	mPUT
	mTRACE
)

var (
	// MALL represents the methodType that can match all methods.
	MALL = mCONNECT | mDELETE | mGET | mHEAD |
		mOPTIONS | mPATCH | mPOST | mPUT | mTRACE

	// Methods represents the mapping of http method and methodType.
	Methods = map[string]MethodType{
		http.MethodGet:     mGET,
		http.MethodHead:    mHEAD,
		http.MethodPost:    mPOST,
		http.MethodPut:     mPUT,
		http.MethodPatch:   mPATCH,
		http.MethodDelete:  mDELETE,
		http.MethodConnect: mCONNECT,
		http.MethodOptions: mOPTIONS,
		http.MethodTrace:   mTRACE,
	}

	kinds = map[string]*Kind{}
)

// Register registers a router kind.
func Register(k *Kind) {
	name := k.Name
	if name == "" {
		panic(fmt.Errorf("%T: empty router name", k))
	}

	if k1 := kinds[k.Name]; k1 != nil {
		msgFmt := "%T and %T got same name: %s"
		panic(fmt.Errorf(msgFmt, k, k1, k.Name))
	}

	kinds[name] = k
}

// Create creates a router instance of kind.
func Create(kind string, rules Rules) Router {
	k := kinds[kind]
	if k == nil {
		return nil
	}
	return k.CreateInstance(rules)
}

// NewContext creates a context instance.
func NewContext(req *httpprot.Request) *RouteContext {
	path := req.Path()

	context := &RouteContext{
		Path:    path,
		Request: req,
		Method:  Methods[req.Method()],
	}

	return context
}

// GetCaptures is used to get and cache path parameter mappings.
func (ctx *RouteContext) GetCaptures() map[string]string {
	if ctx.captures != nil {
		return ctx.captures
	}

	ctx.captures = make(map[string]string)

	if len(ctx.Params.Keys) == 0 {
		return ctx.captures
	}

	for i, key := range ctx.Params.Keys {
		value := ctx.Params.Values[i]
		ctx.captures[key] = value
	}

	return ctx.captures
}

// GetHost is used to get and cache host.
func (ctx *RouteContext) GetHost() string {
	if ctx.host != "" {
		return ctx.host
	}

	host := ctx.Request.Host()
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}

	ctx.host = host
	return host
}

// GetQueries is used to get and cache query params.
func (ctx *RouteContext) GetQueries() url.Values {
	if ctx.queries != nil {
		return ctx.queries
	}
	ctx.queries = ctx.Request.Std().URL.Query()
	return ctx.queries
}

// GetHeader is used to get request http header.
func (ctx *RouteContext) GetHeader() http.Header {
	return ctx.Request.HTTPHeader()
}

// ParseNginxLikeVar parses and substitutes nginx-like variables in the given string.
//
// Supported variable syntaxes:
//   - $var          - Simple variable reference
//   - ${var}        - Braced variable reference
//   - ${var:default} - Variable with default value
//
// Examples:
//   - "$host" -> "example.com"
//   - "${host}-backend" -> "example.com-backend"
//   - "${remote_user:anonymous}" -> "anonymous" (if remote_user is empty)
//   - "$scheme://$host$uri" -> "http://example.com/path"
//   - "$proxy_add_x_forwarded_for" -> "10.0.0.1, 192.168.1.100" (appends client IP to XFF)
//
// Supported variables include request info ($uri, $request_method, $args),
// headers ($http_*), query params ($arg_*), cookies ($cookie_*),
// proxy variables ($proxy_add_x_forwarded_for), and more.
func (ctx *RouteContext) ParseNginxLikeVar(val string) string {
	if val == "" {
		return val
	}

	// Regex patterns for nginx variable syntax
	// Matches $var or ${var} or ${var:default}
	// Updated to handle edge cases better
	varPattern := regexp.MustCompile(`\$\{([^}:]+)(?::([^}]*))?\}|\$([a-zA-Z_][a-zA-Z0-9_]*)`)

	result := varPattern.ReplaceAllStringFunc(val, func(match string) string {
		var varName, defaultValue string

		if strings.HasPrefix(match, "${") {
			// Handle ${var} or ${var:default} syntax
			content := match[2 : len(match)-1] // Remove ${ and }
			parts := strings.SplitN(content, ":", 2)
			varName = parts[0]
			if len(parts) > 1 {
				defaultValue = parts[1]
			}
		} else {
			// Handle $var syntax
			varName = match[1:] // Remove $
		}

		value := ctx.getVariableValue(varName)
		if value == "" && defaultValue != "" {
			return defaultValue
		}
		return value
	})

	return result
}

// getVariableValue returns the value of nginx-like variables
func (ctx *RouteContext) getVariableValue(varName string) string {
	switch varName {
	// Request line variables
	case "request_method":
		return ctx.Request.Method()
	case "request_uri":
		// Build request URI from path and query
		uri := ctx.Request.Path()
		if query := ctx.Request.Std().URL.RawQuery; query != "" {
			uri += "?" + query
		}
		return uri
	case "uri":
		return ctx.Request.Path()
	case "request":
		uri := ctx.Request.Path()
		if query := ctx.Request.Std().URL.RawQuery; query != "" {
			uri += "?" + query
		}
		return fmt.Sprintf("%s %s %s", ctx.Request.Method(), uri, ctx.Request.Proto())
	case "scheme":
		return ctx.Request.Scheme()
	case "query_string":
		return ctx.Request.Std().URL.RawQuery
	case "args":
		return ctx.Request.Std().URL.RawQuery

	// Host variables
	case "host":
		return ctx.GetHost()
	case "hostname":
		return ctx.GetHost()
	case "http_host":
		return ctx.Request.Host()
	case "server_name":
		return ctx.GetHost()

	// Remote address variables
	case "remote_addr":
		return ctx.Request.RealIP()
	case "remote_user":
		// Basic auth user if available
		if username, _, ok := ctx.Request.Std().BasicAuth(); ok {
			return username
		}
		return ""
	case "proxy_add_x_forwarded_for":
		// Builds X-Forwarded-For header value by appending client IP to existing value
		clientIP := ctx.Request.RealIP()
		if clientIP == "" {
			// Fallback to RemoteAddr if RealIP is empty
			if remoteAddr := ctx.Request.Std().RemoteAddr; remoteAddr != "" {
				if host, _, err := net.SplitHostPort(remoteAddr); err == nil {
					clientIP = host
				} else {
					clientIP = remoteAddr
				}
			}
		}

		if clientIP == "" {
			return ""
		}

		// Get existing X-Forwarded-For header
		if existingXFF := ctx.Request.Header().Get("X-Forwarded-For"); existingXFF != nil {
			if xffStr := existingXFF.(string); xffStr != "" {
				return xffStr + ", " + clientIP
			}
		}
		return clientIP

	// Time variables
	case "time_iso8601":
		return time.Now().Format(time.RFC3339)
	case "time_local":
		return time.Now().Format("02/Jan/2006:15:04:05 -0700")
	case "msec":
		return strconv.FormatInt(time.Now().UnixNano()/1000000, 10)

	// Request body and content variables
	case "content_type":
		if v := ctx.Request.Header().Get("Content-Type"); v != nil {
			return v.(string)
		}
		return ""
	case "content_length":
		if v := ctx.Request.Header().Get("Content-Length"); v != nil {
			return v.(string)
		}
		return ""

	// Protocol variables
	case "server_protocol":
		return ctx.Request.Proto()
	case "request_time":
		// This would require tracking request start time
		return "0.000"
	case "request_length":
		if v := ctx.Request.Header().Get("Content-Length"); v != nil {
			return v.(string)
		}
		return "0"

	// HTTP headers (http_*)
	default:
		if strings.HasPrefix(varName, "http_") {
			headerName := strings.ReplaceAll(varName[5:], "_", "-")
			// Normalize header name for standard headers
			headerName = http.CanonicalHeaderKey(headerName)
			if v := ctx.Request.Header().Get(headerName); v != nil {
				return v.(string)
			}
			return ""
		}

		// Query parameters (arg_*)
		if strings.HasPrefix(varName, "arg_") {
			paramName := varName[4:]
			return ctx.GetQueries().Get(paramName)
		}

		// Cookie variables (cookie_*)
		if strings.HasPrefix(varName, "cookie_") {
			cookieName := varName[7:]
			if cookie, err := ctx.Request.Std().Cookie(cookieName); err == nil {
				return cookie.Value
			}
			return ""
		}

		// Path parameters from route captures
		if captures := ctx.GetCaptures(); captures != nil {
			if value, exists := captures[varName]; exists {
				return value
			}
		}

		return ""
	}
}
