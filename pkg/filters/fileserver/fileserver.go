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
	"os"
	"path/filepath"
	"regexp"
	"strconv"
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

var defaultIndexFiles = []string{"index.html", "index.txt"}

var (
	ErrUnsafePath   = errors.New("unsafe path")
	ErrFileNotFound = errors.New("file not found")
)

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

		tryFiles      []string
		tryErr        *Err
		rewrite       *regexp.Regexp
		rewriteTo     string
		precompressed map[string]struct{}
	}

	// Spec describes the FileServer.
	Spec struct {
		filters.BaseSpec `json:",inline"`
		Root             string    `json:"root"`
		Index            []string  `json:"index"`
		Hidden           []string  `json:"hidden"`
		TryFiles         []string  `json:"tryFiles"`
		Rewrite          string    `json:"rewrite"`
		Precompressed    string    `json:"precompressed"`
		Compress         string    `json:"compress"`
		Cache            CacheSpec `json:"cache"`
	}

	CacheSpec struct {
		BufferPoolMaxSize         int                   `json:"bufferPoolSize"`
		BufferPoolMaxFileSize     int                   `json:"bufferPoolMaxFiles"`
		BufferPoolTTL             int                   `json:"bufferPoolTTL"`
		CacheFileExtensionFilters []FileExtensionFilter `json:"cacheFileExtensionFilters"`
	}

	FileExtensionFilter struct {
		Pattern    []string `json:"pattern"`
		EtagMaxAge int      `json:"etagMaxAge"`
	}

	filePath struct {
		path       string
		compressed string
		isHidden   bool
		needCache  bool
		etagMaxAge int
	}
)

func (s *Spec) Validate() error {
	_, err := filepath.Abs(s.Root)
	if err != nil {
		return fmt.Errorf("invalid root path: %v", err)
	}
	if s.Compress != "" && s.Compress != "gzip" {
		return fmt.Errorf("invalid compress type %s", s.Compress)
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
	f.absRoot, _ = filepath.Abs(f.spec.Root)
	for _, h := range f.spec.Hidden {
		if !strings.Contains(h, fileSeparator) {
			f.hiddenWithoutSep = append(f.hiddenWithoutSep, h)
		} else {
			f.hiddenWithSep = append(f.hiddenWithSep, h)
		}
	}
	if len(f.spec.Index) == 0 {
		f.spec.Index = defaultIndexFiles
	}

	for _, t := range f.spec.TryFiles {
		if len(t) > 0 && t[0] == '=' {
			f.tryErr = parseTryFileErr(t)
		} else {
			f.tryFiles = append(f.tryFiles, t)
		}
	}

	if f.spec.Rewrite != "" {
		from, to := parseRewrite(f.spec.Rewrite)
		re := regexp.MustCompile(from)
		f.rewrite = re
		f.rewriteTo = to
	}
	if f.spec.Precompressed != "" {
		f.precompressed = make(map[string]struct{})
		for _, p := range strings.Split(f.spec.Precompressed, " ") {
			f.precompressed[p] = struct{}{}
		}
	}
}

func parseRewrite(rewrite string) (string, string) {
	parts := strings.SplitN(rewrite, " ", 2)
	if len(parts) == 1 {
		return parts[0], ""
	}
	return parts[0], parts[1]
}

func parseTryFileErr(t string) *Err {
	// format: "=404 error messages"
	parts := strings.SplitN(strings.TrimSpace(t), " ", 2)
	code, err := strconv.Atoi(parts[0][1:])
	if err != nil {
		logger.Errorf("invalid try_files error format %s: %v", t, err)
		return nil
	}
	if len(parts) == 1 {
		return &Err{
			Code:    code,
			Message: fmt.Sprintf("%d", code),
		}
	}
	return &Err{
		Code:    code,
		Message: parts[1],
	}
}

// Handle FileServer HTTPContext.
func (f *FileServer) Handle(ctx *context.Context) string {
	req := ctx.GetInputRequest().(*httpprot.Request)
	rw, _ := ctx.GetData("HTTP_RESPONSE_WRITER").(http.ResponseWriter)
	return f.fileHandler(ctx, rw, req.Request, f.pool, f.mc)
}

func (f *FileServer) getFilePath(r *http.Request) (*filePath, error) {
	replacer := strings.NewReplacer(
		"{path}", r.URL.Path,
	)
	path := filepath.Clean(r.URL.Path)
	precompressed := f.getAcceptPrecompressedTypes(r)

	if f.rewrite != nil {
		if f.rewrite.MatchString(path) {
			path = f.rewrite.ReplaceAllString(path, f.rewriteTo)
		}
	}

	if len(f.spec.TryFiles) == 0 {
		return f.convertToFilePathWithExtensionAttribute(path, precompressed)
	}

	for _, t := range f.tryFiles {
		newPath := replacer.Replace(t)
		finalPath, err := f.convertToFilePathWithExtensionAttribute(newPath, precompressed)
		if err == nil {
			return finalPath, nil
		}
	}
	if f.tryErr != nil {
		return nil, f.tryErr
	}
	return nil, ErrFileNotFound
}

func (f *FileServer) convertToFilePathWithExtensionAttribute(path string, precompressed []string) (*filePath, error) {
	preProcessedPath, err := f.convertToFilePath(path, precompressed)
	if err != nil {
		return preProcessedPath, err
	}

	f.setFileHidden(preProcessedPath)
	f.setFileNeedCache(preProcessedPath)
	return preProcessedPath, nil
}

func (f *FileServer) convertToFilePath(path string, precompressed []string) (*filePath, error) {
	finalPath := filepath.Join(f.absRoot, path)
	if !strings.HasPrefix(finalPath, f.absRoot) {
		logger.Errorf("file path %s is not under root path %s", finalPath, f.absRoot)
		return nil, ErrUnsafePath
	}

	if strings.HasSuffix(finalPath, fileSeparator) {
		for _, index := range f.spec.Index {
			newPath := filepath.Join(finalPath, index)

			precompressedPath, ok := checkPrecompressed(newPath, precompressed)
			if ok {
				return precompressedPath, nil
			}

			info, err := os.Stat(newPath)
			if err == nil && !info.IsDir() {
				return &filePath{
					path:       newPath,
					compressed: "",
				}, nil
			}
		}
		logger.Errorf("not find index file under %s", finalPath)
		return nil, ErrFileNotFound
	}

	precompressedPath, ok := checkPrecompressed(finalPath, precompressed)
	if ok {
		return precompressedPath, nil
	}

	info, err := os.Stat(finalPath)
	if err == nil && !info.IsDir() {
		return &filePath{
			path:       finalPath,
			compressed: "",
		}, nil
	}
	logger.Errorf("%s is not a file", finalPath)
	return nil, ErrFileNotFound
}

func checkPrecompressed(path string, precompressed []string) (*filePath, bool) {
	if len(precompressed) == 0 {
		return nil, false
	}
	for _, p := range precompressed {
		precompressedPath := path + "." + p
		info, err := os.Stat(precompressedPath)
		if err == nil && !info.IsDir() {
			return &filePath{
				path:       precompressedPath,
				compressed: p,
			}, true
		}
	}
	return nil, false
}

func (f *FileServer) fileHandler(ctx *context.Context, w http.ResponseWriter, r *http.Request, pool *BufferPool, mc *MmapCache) string {
	startTime := fasttime.Now()

	path, err := f.getFilePath(r)
	if err != nil {
		buildFailureRespWithErr(ctx, err)
		return resultClientError
	}
	f.setFileHidden(path)
	if path.isHidden {
		buildFailureResponse(ctx, http.StatusForbidden, "access denied")
		return resultClientError
	}

	data, _, err := pool.GetFile(path.path)
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

func buildFailureRespWithErr(ctx *context.Context, err error) {
	if errors.Is(err, ErrUnsafePath) {
		buildFailureResponse(ctx, http.StatusBadRequest, "bad request")

	} else if errors.Is(err, ErrFileNotFound) {
		buildFailureResponse(ctx, http.StatusNotFound, "file not found")

	} else if e, ok := err.(*Err); ok {
		buildFailureResponse(ctx, e.Code, e.Message)

	} else {
		buildFailureResponse(ctx, http.StatusInternalServerError, "internal server error")
	}
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

func (f *FileServer) getAcceptPrecompressedTypes(req *http.Request) []string {
	if len(f.precompressed) == 0 {
		return nil
	}
	accepts := req.Header.Values("Accept-Encoding")
	var res []string
	for _, a := range accepts {
		if _, ok := f.precompressed[a]; ok {
			res = append(res, a)
		}
	}
	return res
}

// setFileHidden determines if a given absolute path should be considered hidden.
// It checks the path against a list of predefined rules. These rules are
// pre-processed and split into two categories for matching:
//
//  1. Component Patterns (rules without a path separator):
//     These patterns (e.g., ".git", "*.log") are matched against each individual
//     name component of the relative path.
//     - Rule: ".git"
//     - Hides: "/path/to/project/.git", "/path/to/.git/config"
//     - Does NOT hide: "/path/to/project/git"
//     - Rule: "node_modules"
//     - Hides: "/path/to/project/node_modules", "/path/to/node_modules/express"
//
//  2. Path Patterns (rules with a path separator):
//     These patterns are matched against the full absolute path. This matching
//     is done in two ways:
//     a) As a prefix: If the rule is a prefix of the path, followed by a
//     path separator, it's a match.
//     - Rule: "/build"
//     - Hides: "/build/app.js", "/build/"
//     - Does NOT hide: "/builder/app.js"
//     b) As a glob pattern: The rule is treated as a glob pattern to be matched
//     against the entire absolute path.
//     - Rule: "/*/*/*.conf"
//     - Hides: "/etc/nginx/nginx.conf"
//     - Does NOT hide: "/etc/nginx.conf"
func (f *FileServer) setFileHidden(absPath *filePath) {
	if len(f.spec.Hidden) == 0 {
		absPath.isHidden = false
		return
	}

	if len(f.hiddenWithoutSep) > 0 {
		path := strings.TrimPrefix(absPath.path, f.absRoot)
		parts := strings.Split(path, fileSeparator)
		for _, h := range f.hiddenWithoutSep {
			for _, c := range parts {
				if hidden, _ := filepath.Match(h, c); hidden {
					absPath.isHidden = true
					return
				}
			}
		}
	}

	for _, h := range f.hiddenWithSep {
		if remain, ok := strings.CutPrefix(absPath.path, h); ok {
			if strings.HasPrefix(remain, fileSeparator) || strings.HasSuffix(h, fileSeparator) {
				absPath.isHidden = true
				return
			}
		}

		if hidden, _ := filepath.Match(h, absPath.path); hidden {
			absPath.isHidden = true
			return
		}
	}
	absPath.isHidden = false
	return
}

func (f *FileServer) setFileNeedCache(fp *filePath) {
	if len(f.spec.Cache.CacheFileExtensionFilters) == 0 {
		fp.needCache = false
		return
	}

	for _, filter := range f.spec.Cache.CacheFileExtensionFilters {
		for _, pattern := range filter.Pattern {
			realPattern := pattern
			targetPath := fp.path
			if strings.Contains(realPattern, fileSeparator) {
				if !strings.HasPrefix(realPattern, fileSeparator) && strings.HasPrefix(targetPath, fileSeparator) {
					realPattern = fileSeparator + realPattern
				}
			} else {
				targetPath = filepath.Base(targetPath)
			}
			if matched, _ := filepath.Match(realPattern, targetPath); matched {
				fp.needCache = true
				fp.etagMaxAge = filter.EtagMaxAge
				if fp.etagMaxAge <= 0 {
					fp.etagMaxAge = DefaultEtagMaxAge
				}
				return
			}
		}
	}
	fp.needCache = false
}
