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
	"bytes"
	"compress/gzip"
	"fmt"
	"net/http"
	"slices"
	"strings"
)

type gzipResponseWriter struct {
	http.ResponseWriter
	gzWriter *gzip.Writer
}

func newGzipResponseWriter(w http.ResponseWriter) *gzipResponseWriter {
	gz := gzip.NewWriter(w)
	return &gzipResponseWriter{
		ResponseWriter: w,
		gzWriter:       gz,
	}
}

func (gzw *gzipResponseWriter) Write(data []byte) (int, error) {
	return gzw.gzWriter.Write(data)
}

func (gzw *gzipResponseWriter) Close() error {
	return gzw.gzWriter.Close()
}

func isRangeRequest(r *http.Request) bool {
	rangeHeader := r.Header.Get("Range")
	return strings.HasPrefix(rangeHeader, "bytes=")
}

func (f *FileServer) shouldCompress(r *http.Request, filePath *filePath) bool {
	if f.spec.Compress == "" {
		return false
	}
	if filePath.compressed != "" {
		return false
	}
	if isRangeRequest(r) {
		return false
	}
	accepts := r.Header.Values("Accept-Encoding")
	if len(accepts) == 0 {
		return false
	}
	return slices.Contains(accepts, "gzip")
}

func compressData(data []byte) ([]byte, error) {
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	_, err := gz.Write(data)
	if err != nil {
		return nil, fmt.Errorf("failed to write data to gzip writer: %w", err)
	}
	if err := gz.Close(); err != nil {
		return nil, fmt.Errorf("failed to close gzip writer: %w", err)
	}
	return b.Bytes(), nil
}
