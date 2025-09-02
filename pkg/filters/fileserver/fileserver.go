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

// Package fileserver implements the fileserver filter.
package fileserver

import (
	"net/http"

	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/filters"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot"
)

const (
	// Kind is the kind of FileServer.
	Kind = "FileServer"

	resultInternalError = "internalError"
	resultServerError   = "serverError"
	resultClientError   = "clientError"
	resultNotFound      = "notFound"

	DefaultEtagMaxAge = 3600
)

var kind = &filters.Kind{
	Name:        Kind,
	Description: "FileServer do the file server.",
	Results:     []string{resultInternalError, resultServerError, resultClientError},
	DefaultSpec: func() filters.Spec {
		return &Spec{}
	},
	CreateInstance: func(spec filters.Spec) filters.Filter {
		return &FileServer{spec: spec.(*Spec)}
	},
}

func init() {
	filters.Register(kind)
}

type (
	// FileServer is filter FileServer.
	FileServer struct {
		spec *Spec
		pool *BufferPool
		mc   *MmapCache
	}

	// Spec describes the FileServer.
	Spec struct {
		filters.BaseSpec `json:",inline"`
		EtagMaxAge       int `json:"etagMaxAge"`
	}
)

// Name returns the name of the FileServer filter instance.
func (f *FileServer) Name() string {
	return f.spec.Name()
}

// Kind returns the kind of FileServer.
func (f *FileServer) Kind() *filters.Kind {
	return kind
}

// Spec returns the spec used by the FileServer
func (f *FileServer) Spec() filters.Spec {
	return f.spec
}

// Init initializes FileServer.
func (f *FileServer) Init() {
	f.reload()
}

// Inherit inherits previous generation of FileServer.
func (f *FileServer) Inherit(previousGeneration filters.Filter) {
	f.Init()
}

func (f *FileServer) reload() {
	if f.spec.EtagMaxAge == 0 {
		f.spec.EtagMaxAge = DefaultEtagMaxAge
	}
}

// Handle FileServer HTTPContext.
func (f *FileServer) Handle(ctx *context.Context) string {
	req := ctx.GetInputRequest().(*httpprot.Request)
	rw, _ := ctx.GetData("HTTP_RESPONSE_WRITER").(http.ResponseWriter)
	return f.fileHandler(ctx, rw, req.Request, f.pool, f.mc)
}

// Status returns Status.
func (f *FileServer) Status() interface{} {
	return nil
}

// Close closes FileServer.
func (f *FileServer) Close() {
}

func buildFailureResponse(ctx *context.Context, statusCode int, message string) {
	resp, _ := ctx.GetOutputResponse().(*httpprot.Response)
	if resp == nil {
		resp, _ = httpprot.NewResponse(nil)
	}

	resp.SetStatusCode(statusCode)
	resp.SetPayload([]byte(message))
	ctx.SetOutputResponse(resp)
}
