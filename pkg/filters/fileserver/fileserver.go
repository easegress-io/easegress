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
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/filters"
	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot"
	"github.com/megaease/easegress/v2/pkg/util/fasttime"
)

const (
	// Kind is the kind of FileServer.
	Kind = "FileServer"

	resultInternalError = "internalError"
	resultServerError   = "serverError"
	resultClientError   = "clientError"
	resultNotFound      = "notFound"

	DefaultEtagMaxAge = 3600
	fileSeparator     = string(filepath.Separator)
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

		absRoot          string
		hiddenWithSep    []string
		hiddenWithoutSep []string
	}

	// Spec describes the FileServer.
	Spec struct {
		filters.BaseSpec `json:",inline"`
		EtagMaxAge       int      `json:"etagMaxAge"`
		Root             string   `json:"root"`
		Hidden           []string `json:"hidden"`
	}
)

func (s *Spec) Validate() error {
	_, err := filepath.Abs(s.Root)
	if err != nil {
		return fmt.Errorf("invalid root path: %v", err)
	}
	return nil
}

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

	f.absRoot, _ = filepath.Abs(f.spec.Root)
	for _, h := range f.spec.Hidden {
		if !strings.Contains(h, fileSeparator) {
			f.hiddenWithoutSep = append(f.hiddenWithoutSep, h)
		} else {
			f.hiddenWithSep = append(f.hiddenWithSep, h)
		}
	}
}

// Handle FileServer HTTPContext.
func (f *FileServer) Handle(ctx *context.Context) string {
	req := ctx.GetInputRequest().(*httpprot.Request)
	rw, _ := ctx.GetData("HTTP_RESPONSE_WRITER").(http.ResponseWriter)
	return f.fileHandler(ctx, rw, req.Request, f.pool, f.mc)
}

func (f *FileServer) getFilePath(r *http.Request) (string, error) {
	path := r.URL.Path

	// TODO try files
	finalPath := filepath.Join(f.absRoot, path)

	if !strings.HasPrefix(finalPath, f.absRoot) {
		logger.Errorf("file path %s is not under root path %s", finalPath, f.absRoot)
		return "", errors.New("unsafe path: potential path traversal attack")
	}
	return finalPath, nil
}

func (f *FileServer) fileHandler(ctx *context.Context, w http.ResponseWriter, r *http.Request, pool *BufferPool, mc *MmapCache) string {
	startTime := fasttime.Now()

	path, err := f.getFilePath(r)
	if err != nil {
		buildFailureResponse(ctx, http.StatusBadRequest, "bad request")
		return resultClientError
	}
	hidden := f.isFileHidden(path)
	if hidden {
		buildFailureResponse(ctx, http.StatusForbidden, "access denied")
		return resultClientError
	}

	data, _, err := pool.GetFile(path)
	if err != nil {
		buildFailureResponse(ctx, http.StatusNotFound, "file not found")
		return resultNotFound
	}
	if data != nil {
		return f.handleWithSmallFile(ctx, w, r, path, data, startTime)
	}

	return f.handleWithLargeFile(ctx, w, r, path, startTime, mc)
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

// isFileHidden check if the file is hidden.
func (f *FileServer) isFileHidden(absPath string) bool {
	if len(f.spec.Hidden) == 0 {
		return false
	}

	if len(f.hiddenWithoutSep) > 0 {
		path := strings.TrimPrefix(absPath, f.absRoot)
		parts := strings.Split(path, fileSeparator)
		for _, h := range f.hiddenWithoutSep {
			for _, c := range parts {
				if hidden, _ := filepath.Match(h, c); hidden {
					return true
				}
			}
		}
	}

	for _, h := range f.hiddenWithSep {
		if remain, ok := strings.CutPrefix(absPath, h); ok {
			if strings.HasPrefix(remain, fileSeparator) || strings.HasSuffix(h, fileSeparator) {
				return true
			}
		}

		if hidden, _ := filepath.Match(h, absPath); hidden {
			return true
		}
	}
	return false
}
